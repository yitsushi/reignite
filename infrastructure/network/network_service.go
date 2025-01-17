package network

import (
	"context"
	ierror "errors"
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"

	"github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/log"
)

type Config struct {
	ParentDeviceName string
}

func New(cfg *Config) ports.NetworkService {
	return &networkService{
		parentDeviceName: cfg.ParentDeviceName,
	}
}

type networkService struct {
	parentDeviceName string
}

// IfaceCreate will create the network interface.
func (n *networkService) IfaceCreate(ctx context.Context, input ports.IfaceCreateInput) (*ports.IfaceDetails, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "netlink_network",
		"iface":   input.DeviceName,
	})
	logger.Debugf(
		"creating network interface with type %s and MAC %s using parent %s",
		input.Type,
		input.MAC,
		n.parentDeviceName,
	)

	var (
		parentLink netlink.Link
		err        error
	)

	if input.Type == models.IfaceTypeMacvtap {
		if n.parentDeviceName == "" {
			return nil, errors.ErrParentIfaceRequired
		}

		parentLink, err = netlink.LinkByName(n.parentDeviceName)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup parent network interface %q: %w", n.parentDeviceName, err)
		}
	}

	var link netlink.Link

	switch input.Type {
	case models.IfaceTypeTap:
		link = &netlink.Tuntap{
			LinkAttrs: netlink.LinkAttrs{
				Name: input.DeviceName,
				// TODO: add Namespace #206
			},
			Mode: netlink.TUNTAP_MODE_TAP,
		}
	case models.IfaceTypeMacvtap:
		link = &netlink.Macvtap{
			Macvlan: netlink.Macvlan{
				LinkAttrs: netlink.LinkAttrs{
					Name:        input.DeviceName,
					MTU:         parentLink.Attrs().MTU,
					ParentIndex: parentLink.Attrs().Index,
					Namespace:   parentLink.Attrs().Namespace, // TODO: add namespace specific to vm #206
					TxQLen:      parentLink.Attrs().TxQLen,
				},
				Mode: netlink.MACVLAN_MODE_BRIDGE,
			},
		}

		if input.MAC != "" {
			addr, parseErr := net.ParseMAC(input.MAC)
			if err != nil {
				return nil, fmt.Errorf("parsing mac address %s: %w", input.MAC, parseErr)
			}

			link.Attrs().HardwareAddr = addr
			logger.Tracef("added mac address %s to interface", addr)
		}
	case models.IfaceTypeUnsupported:
		return nil, errors.NewErrUnsupportedInterface(string(input.Type))
	default:
		return nil, errors.NewErrUnsupportedInterface(string(input.Type))
	}

	if err = netlink.LinkAdd(link); err != nil {
		return nil, fmt.Errorf("creating interface %s using netlink: %w", link.Attrs().Name, err)
	}

	macIf, err := netlink.LinkByName(link.Attrs().Name)
	if err != nil {
		return nil, fmt.Errorf("getting interface %s using netlink: %w", link.Attrs().Name, err)
	}

	if err := netlink.LinkSetUp(macIf); err != nil {
		return nil, fmt.Errorf("enabling device %s: %w", macIf.Attrs().Name, err)
	}

	logger.Debugf("created interface with mac %s", macIf.Attrs().HardwareAddr.String())

	return &ports.IfaceDetails{
		DeviceName: input.DeviceName,
		Type:       input.Type,
		MAC:        strings.ToUpper(macIf.Attrs().HardwareAddr.String()),
		Index:      macIf.Attrs().Index,
	}, nil
}

// IfaceDelete is used to delete a network interface.
func (n *networkService) IfaceDelete(ctx context.Context, input ports.DeleteIfaceInput) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "netlink_network",
		"iface":   input.DeviceName,
	})
	logger.Debug("deleting network interface")

	link, err := netlink.LinkByName(input.DeviceName)
	if err != nil {
		if ierror.Is(err, netlink.LinkNotFoundError{}) {
			return fmt.Errorf("failed to lookup network interface %s: %w", input.DeviceName, err)
		}

		logger.Debug("network interface doesn't exist, no action")

		return nil
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("deleting interface %s: %w", link.Attrs().Name, err)
	}

	return nil
}

func (n *networkService) IfaceExists(ctx context.Context, name string) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "netlink_network",
		"iface":   name,
	})
	logger.Debug("checking if network interface exists")

	found, _, err := n.getIface(name)
	if err != nil {
		return false, fmt.Errorf("getting interface %s: %w", name, err)
	}

	return found, nil
}

// IfaceDetails will get the details of the supplied network interface.
func (n *networkService) IfaceDetails(ctx context.Context, name string) (*ports.IfaceDetails, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "netlink_network",
		"iface":   name,
	})
	logger.Debug("getting network interface details")

	found, link, err := n.getIface(name)
	if err != nil {
		return nil, fmt.Errorf("getting interface %s: %w", name, err)
	}

	if !found {
		return nil, errors.ErrIfaceNotFound
	}

	details := &ports.IfaceDetails{
		DeviceName: name,
		MAC:        strings.ToUpper(link.Attrs().HardwareAddr.String()),
		Index:      link.Attrs().Index,
	}

	switch link.(type) {
	case *netlink.Macvtap:
		details.Type = models.IfaceTypeMacvtap
	case *netlink.Tuntap:
		details.Type = models.IfaceTypeTap
	default:
		details.Type = models.IfaceTypeUnsupported
	}

	return details, nil
}

func (n *networkService) getIface(name string) (bool, netlink.Link, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		if ierror.Is(err, netlink.LinkNotFoundError{}) {
			return false, nil, fmt.Errorf("failed to lookup network interface %s: %w", name, err)
		}

		return false, nil, nil
	}

	return true, link, nil
}
