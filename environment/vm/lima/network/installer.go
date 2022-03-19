package network

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
)

type rootfulInstaller struct{ host environment.HostActions }

func (r rootfulInstaller) Installed(file rootfulFile) bool {
	stat, err := os.Stat(file.Path())
	if err != nil {
		return false
	}
	if file.Executable() {
		return stat.Mode()&0100 != 0
	}
	return true
}
func (r rootfulInstaller) Install(file rootfulFile) error { return file.Install(r.host) }

type rootfulFile interface {
	Path() string
	Executable() bool
	Install(host environment.HostActions) error
}

var _ rootfulFile = sudoerFile{}

type sudoerFile struct{}

func (s sudoerFile) Path() string     { return "/etc/sudoers.d/colima" }
func (s sudoerFile) Executable() bool { return false }
func (s sudoerFile) Install(host environment.HostActions) error {
	// read embedded file contents
	txt, err := embedded.ReadString("network/sudo.txt")
	if err != nil {
		return fmt.Errorf("error retrieving embedded sudo file: %w", err)
	}
	// ensure parent directory exists
	if err := host.RunInteractive("sudo", "mkdir", "-p", filepath.Dir(s.Path())); err != nil {
		return fmt.Errorf("error preparing sudoers directory: %w", err)
	}
	// persist file to desired location
	if err := host.RunInteractive("sudo", "sh", "-c", fmt.Sprintf(`echo "%s" > %s`, txt, s.Path())); err != nil {
		return fmt.Errorf("error writing sudoers file: %w", err)
	}
	return nil
}

var _ rootfulFile = vmnetFile{}

const VmnetBinary = "/opt/colima/bin/vde_vmnet"

type vmnetFile struct{}

func (s vmnetFile) Path() string     { return VmnetBinary }
func (s vmnetFile) Executable() bool { return true }
func (s vmnetFile) Install(host environment.HostActions) error {
	arch := "x86_64"
	if runtime.GOARCH != "amd64" {
		arch = "arm64"
	}

	// read the embedded file
	gz, err := embedded.Read("network/vmnet_" + arch + ".tar.gz")
	if err != nil {
		return fmt.Errorf("error retrieving embedded vmnet file: %w", err)
	}

	// write tar to tmp directory
	f, err := os.CreateTemp("", "vmnet.tar.gz")
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	if _, err := f.Write(gz); err != nil {
		return fmt.Errorf("error writing temp file: %w", err)
	}
	_ = f.Close() // not a fatal error

	// extract tar to desired location
	dir := filepath.Dir(s.Path())
	if err := host.RunInteractive("sudo", "mkdir", "-p", dir); err != nil {
		return fmt.Errorf("error preparing colima privileged dir: %w", err)
	}
	if err := host.RunInteractive("sudo", "sh", "-c", fmt.Sprintf("cd %s && tar xfz %s", dir, f.Name())); err != nil {
		return fmt.Errorf("error extracting vmnet archive: %w", err)
	}
	return nil
}

var _ rootfulFile = colimaVmnetFile{}

type colimaVmnetFile struct{}

func (s colimaVmnetFile) Path() string     { return "/opt/colima/bin/colima-vmnet" }
func (s colimaVmnetFile) Executable() bool { return true }
func (s colimaVmnetFile) Install(host environment.HostActions) error {
	arg0, _ := exec.LookPath(os.Args[0])
	if arg0 == "" { // should never happen
		arg0 = os.Args[0]
	}
	if err := host.RunInteractive("sudo", "ln", "-sfn", arg0, s.Path()); err != nil {
		return fmt.Errorf("error creating colima-vmnet binary: %w", err)
	}
	return nil
}