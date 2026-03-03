package client

import "context"

type VMState string

const (
	VMStateRunning VMState = "Running"
	VMStateOff     VMState = "Off"
)

type VMOptions struct {
	Name                 string
	Generation           int
	ProcessorCount       int
	MemoryStartupBytes   int64
	MemoryMinimumBytes   int64
	MemoryMaximumBytes   int64
	DynamicMemory        bool
	State                VMState
	Notes                string
	AutomaticStartAction string
	AutomaticStopAction  string
	CheckpointType       string
}

type VM struct {
	Name                 string `json:"Name"`
	Generation           int    `json:"Generation"`
	ProcessorCount       int    `json:"ProcessorCount"`
	MemoryStartup        int64  `json:"MemoryStartup"`
	MemoryMinimum        int64  `json:"MemoryMinimum"`
	MemoryMaximum        int64  `json:"MemoryMaximum"`
	DynamicMemoryEnabled bool   `json:"DynamicMemoryEnabled"`
	State                string `json:"State"`
	Notes                string `json:"Notes"`
	AutomaticStartAction string `json:"AutomaticStartAction"`
	AutomaticStopAction  string `json:"AutomaticStopAction"`
	CheckpointType       string `json:"CheckpointType"`
}

type VHDOptions struct {
	Path       string
	SizeBytes  int64
	Type       string // Dynamic, Fixed, Differencing
	ParentPath string
	BlockSize  int64
}

type VHD struct {
	Path       string `json:"Path"`
	Size       int64  `json:"Size"`
	VhdType    int    `json:"VhdType"`
	ParentPath string `json:"ParentPath"`
	BlockSize  int64  `json:"BlockSize"`
}

type SwitchOptions struct {
	Name                string
	SwitchType          string // External, Internal, Private
	NetAdapterName      string
	AllowManagementOS   bool
	AllowManagementOSSet bool // true if AllowManagementOS was explicitly set by user
}

type VirtualSwitch struct {
	Name               string `json:"Name"`
	SwitchType         int    `json:"SwitchType"`
	NetAdapterName     string `json:"NetAdapterInterfaceDescription"`
	AllowManagementOS  bool   `json:"AllowManagementOS"`
}

type ISOOptions struct {
	Path        string
	VolumeLabel string
	Files       map[string]string
}

type ISOInfo struct {
	Path   string `json:"Path"`
	Exists bool   `json:"Exists"`
	Size   int64  `json:"Size"`
}

type DVDDriveOptions struct {
	VMName                string
	Path                  string
	ControllerNumber      int
	ControllerLocation    int
	ControllerLocationSet bool
}

type DVDDrive struct {
	VMName             string `json:"VMName"`
	Path               string `json:"Path"`
	ControllerNumber   int    `json:"ControllerNumber"`
	ControllerLocation int    `json:"ControllerLocation"`
}

type HardDriveOptions struct {
	VMName                string
	Path                  string
	ControllerType        string // IDE or SCSI
	ControllerNumber      int
	ControllerLocation    int
	ControllerLocationSet bool
}

type HardDrive struct {
	VMName             string `json:"VMName"`
	Path               string `json:"Path"`
	ControllerType     int    `json:"ControllerType"` // 0=IDE, 1=SCSI
	ControllerNumber   int    `json:"ControllerNumber"`
	ControllerLocation int    `json:"ControllerLocation"`
}

type BootDevice struct {
	DeviceType         string `json:"DeviceType"`
	ControllerNumber   int    `json:"ControllerNumber"`
	ControllerLocation int    `json:"ControllerLocation"`
}

type VMFirmwareOptions struct {
	SecureBootEnabled  *bool       // nil = don't change
	SecureBootTemplate string      // "" = don't change
	FirstBootDevice    *BootDevice // nil = don't change
}

type VMFirmware struct {
	SecureBootEnabled                 string `json:"SecureBootEnabled"`  // "On" or "Off"
	SecureBootTemplate                string `json:"SecureBootTemplate"`
	FirstBootDeviceType               string `json:"FirstBootDeviceType"`
	FirstBootDeviceControllerNumber   int    `json:"FirstBootDeviceControllerNumber"`
	FirstBootDeviceControllerLocation int    `json:"FirstBootDeviceControllerLocation"`
}

type AdapterOptions struct {
	Name              string
	VMName            string
	SwitchName        string
	VlanID            int
	VlanIDSet         bool // true if VlanID was explicitly set by user
	MacAddress        string
	DynamicMacAddress bool
}

type NetworkAdapter struct {
	Name              string `json:"Name"`
	VMName            string `json:"VMName"`
	SwitchName        string `json:"SwitchName"`
	VlanID            int    `json:"VlanID"`
	MacAddress        string `json:"MacAddress"`
	DynamicMacAddress bool   `json:"DynamicMacAddressEnabled"`
}

// Compile-time check that WinRMClient implements HyperVClient.
var _ HyperVClient = (*WinRMClient)(nil)

type HyperVClient interface {
	CreateVM(ctx context.Context, opts VMOptions) (*VM, error)
	GetVM(ctx context.Context, name string) (*VM, error)
	UpdateVM(ctx context.Context, name string, opts VMOptions) error
	DeleteVM(ctx context.Context, name string) error
	SetVMState(ctx context.Context, name string, state VMState) error
	SetVMFirmware(ctx context.Context, name string, opts VMFirmwareOptions) error
	GetVMFirmware(ctx context.Context, name string) (*VMFirmware, error)
	SetVMFirstBootDevice(ctx context.Context, name string, device BootDevice) error

	CreateVHD(ctx context.Context, opts VHDOptions) (*VHD, error)
	GetVHD(ctx context.Context, path string) (*VHD, error)
	DeleteVHD(ctx context.Context, path string) error

	CreateVirtualSwitch(ctx context.Context, opts SwitchOptions) (*VirtualSwitch, error)
	GetVirtualSwitch(ctx context.Context, name string) (*VirtualSwitch, error)
	UpdateVirtualSwitch(ctx context.Context, name string, opts SwitchOptions) error
	DeleteVirtualSwitch(ctx context.Context, name string) error

	CreateNetworkAdapter(ctx context.Context, opts AdapterOptions) (*NetworkAdapter, error)
	GetNetworkAdapter(ctx context.Context, vmName, name string) (*NetworkAdapter, error)
	UpdateNetworkAdapter(ctx context.Context, vmName, name string, opts AdapterOptions) error
	DeleteNetworkAdapter(ctx context.Context, vmName, name string) error

	CreateISO(ctx context.Context, opts ISOOptions) (*ISOInfo, error)
	GetISO(ctx context.Context, path string) (*ISOInfo, error)
	DeleteISO(ctx context.Context, path string) error

	CreateDVDDrive(ctx context.Context, opts DVDDriveOptions) (*DVDDrive, error)
	GetDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int) (*DVDDrive, error)
	UpdateDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int, opts DVDDriveOptions) error
	DeleteDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int) error
	ListDVDDrives(ctx context.Context, vmName string) ([]DVDDrive, error)

	CreateHardDrive(ctx context.Context, opts HardDriveOptions) (*HardDrive, error)
	GetHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int) (*HardDrive, error)
	UpdateHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int, path string) error
	DeleteHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int) error
	ListHardDrives(ctx context.Context, vmName string) ([]HardDrive, error)
}
