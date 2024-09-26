//go:build (amd64 && !remote) || (arm64 && !remote)

package system

import (
	"github.com/containers/podman/v5/pkg/machine/connection"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	p "github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/utils"
	"github.com/sirupsen/logrus"
)

func resetMachine() error {
	provider, err := p.Get()
	if err != nil {
		return err
	}
	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}

	mcs, err := vmconfigs.LoadMachinesInDir(dirs)
	if err != nil {
		// Note: the reason we might be cleaning is because a JSON file is messed
		// up and is unreadable.  This should not be fatal.  Keep going ...
		logrus.Errorf("unable to load machines: %q", err)
	}

	machines, err := p.GetAllMachinesAndRootfulness()
	if err != nil {
		return err
	}

	for _, mc := range mcs {
		state, err := provider.State(mc, false)
		if err != nil {
			logrus.Errorf("unable to determine state of %s: %q", mc.Name, err)
		}

		if state == define.Running {
			if err := shim.Stop(mc, provider, dirs, true); err != nil {
				logrus.Errorf("unable to stop running machine %s: %q", mc.Name, err)
			}
		}

		if err := connection.RemoveConnections(machines, mc.Name, mc.Name+"-root"); err != nil {
			logrus.Error(err)
		}

		// the thinking here is that the we dont need to remove machine specific files because
		// we will nuke them all at the end of this.  Just do what provider needs
		_, providerRm, err := provider.Remove(mc)
		if err != nil {
			logrus.Errorf("unable to prepare provider machine removal: %q", err)
		}

		if err := providerRm(); err != nil {
			logrus.Errorf("unable remove machine %s from provider: %q", mc.Name, err)
		}
	}

	if err := utils.GuardedRemoveAll(dirs.DataDir.GetPath()); err != nil {
		logrus.Errorf("unable to remove machine data dir %q: %q", dirs.DataDir.GetPath(), err)
	}

	if err := utils.GuardedRemoveAll(dirs.RuntimeDir.GetPath()); err != nil {
		logrus.Errorf("unable to remove machine runtime dir %q: %q", dirs.RuntimeDir.GetPath(), err)
	}

	if err := utils.GuardedRemoveAll(dirs.ConfigDir.GetPath()); err != nil {
		logrus.Errorf("unable to remove machine config dir %q: %q", dirs.ConfigDir.GetPath(), err)
	}

	// Just in case a provider needs something general done
	return provider.RemoveAndCleanMachines(dirs)
}
