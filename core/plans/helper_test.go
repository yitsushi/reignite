package plans_test

import (
	"time"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/infrastructure/mock"
)

const (
	testUID = "ae1ce196-6249-11ec-90d6-0242ac120003"
)

type mockList struct {
	MicroVMRepository *mock.MockMicroVMRepository
	EventService      *mock.MockEventService
	IDService         *mock.MockIDService
	MicroVMService    *mock.MockMicroVMService
	NetworkService    *mock.MockNetworkService
	ImageService      *mock.MockImageService
}

func fakePorts(mockCtrl *gomock.Controller) (*mockList, *ports.Collection) {
	mList := &mockList{
		MicroVMRepository: mock.NewMockMicroVMRepository(mockCtrl),
		EventService:      mock.NewMockEventService(mockCtrl),
		IDService:         mock.NewMockIDService(mockCtrl),
		MicroVMService:    mock.NewMockMicroVMService(mockCtrl),
		NetworkService:    mock.NewMockNetworkService(mockCtrl),
		ImageService:      mock.NewMockImageService(mockCtrl),
	}

	return mList, &ports.Collection{
		Repo:              mList.MicroVMRepository,
		EventService:      mList.EventService,
		IdentifierService: mList.IDService,
		Provider:          mList.MicroVMService,
		NetworkService:    mList.NetworkService,
		ImageService:      mList.ImageService,
		FileSystem:        afero.NewMemMapFs(),
		Clock:             time.Now,
	}
}

func createTestSpec(name, ns string) *models.MicroVM {
	var vmid *models.VMID

	if name == "" && ns == "" {
		vmid = &models.VMID{}
	} else {
		vmid, _ = models.NewVMID(name, ns, testUID)
	}

	return &models.MicroVM{
		ID: *vmid,
		Status: models.MicroVMStatus{
			State: models.PendingState,
			NetworkInterfaces: models.NetworkInterfaceStatuses{
				"eth0": &models.NetworkInterfaceStatus{
					HostDeviceName: "fltap5675122",
					Index:          0,
				},
			},
		},
		Spec: models.MicroVMSpec{
			VCPU:       2,
			MemoryInMb: 2048,
			Kernel: models.Kernel{
				Image:    "docker.io/linuxkit/kernel:5.4.129",
				Filename: "kernel",
			},
			NetworkInterfaces: []models.NetworkInterface{
				{
					AllowMetadataRequests: true,
					Type:                  models.IfaceTypeTap,
					GuestMAC:              "AA:FF:00:00:00:01",
					GuestDeviceName:       "eth0",
				},
				{
					Type:                  models.IfaceTypeTap,
					AllowMetadataRequests: false,
					GuestDeviceName:       "eth1",
				},
			},
			RootVolume: models.Volume{
				ID:         "root",
				IsReadOnly: false,
				MountPoint: "/",
				Source: models.VolumeSource{
					Container: &models.ContainerVolumeSource{
						Image: "docker.io/library/ubuntu:myimage",
					},
				},
				Size: 20000,
			},
			CreatedAt: 1,
			UpdatedAt: 0,
			DeletedAt: 0,
		},
	}
}
