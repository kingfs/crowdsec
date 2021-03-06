package main

import (
	"fmt"
	"io/ioutil"

	"github.com/crowdsecurity/crowdsec/pkg/cwhub"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func addToExclusion(name string) error {
	csConfig.Crowdsec.SimulationConfig.Exclusions = append(csConfig.Crowdsec.SimulationConfig.Exclusions, name)
	return nil
}

func removeFromExclusion(name string) error {
	index := indexOf(name, csConfig.Crowdsec.SimulationConfig.Exclusions)

	// Remove element from the slice
	csConfig.Crowdsec.SimulationConfig.Exclusions[index] = csConfig.Crowdsec.SimulationConfig.Exclusions[len(csConfig.Crowdsec.SimulationConfig.Exclusions)-1]
	csConfig.Crowdsec.SimulationConfig.Exclusions[len(csConfig.Crowdsec.SimulationConfig.Exclusions)-1] = ""
	csConfig.Crowdsec.SimulationConfig.Exclusions = csConfig.Crowdsec.SimulationConfig.Exclusions[:len(csConfig.Crowdsec.SimulationConfig.Exclusions)-1]

	return nil
}

func enableGlobalSimulation() error {
	csConfig.Crowdsec.SimulationConfig.Simulation = new(bool)
	*csConfig.Crowdsec.SimulationConfig.Simulation = true
	csConfig.Crowdsec.SimulationConfig.Exclusions = []string{}

	if err := dumpSimulationFile(); err != nil {
		log.Fatalf("unable to dump simulation file: %s", err.Error())
	}

	log.Printf("global simulation: enabled")

	return nil
}

func dumpSimulationFile() error {
	newConfigSim, err := yaml.Marshal(csConfig.Crowdsec.SimulationConfig)
	if err != nil {
		return fmt.Errorf("unable to marshal simulation configuration: %s", err)
	}
	err = ioutil.WriteFile(csConfig.ConfigPaths.SimulationFilePath, newConfigSim, 0644)
	if err != nil {
		return fmt.Errorf("write simulation config in '%s' failed: %s", csConfig.ConfigPaths.SimulationFilePath, err)
	}
	log.Debugf("updated simulation file %s", csConfig.ConfigPaths.SimulationFilePath)

	return nil
}

func disableGlobalSimulation() error {
	csConfig.Crowdsec.SimulationConfig.Simulation = new(bool)
	*csConfig.Crowdsec.SimulationConfig.Simulation = false

	csConfig.Crowdsec.SimulationConfig.Exclusions = []string{}
	newConfigSim, err := yaml.Marshal(csConfig.Crowdsec.SimulationConfig)
	if err != nil {
		return fmt.Errorf("unable to marshal new simulation configuration: %s", err)
	}
	err = ioutil.WriteFile(csConfig.ConfigPaths.SimulationFilePath, newConfigSim, 0644)
	if err != nil {
		return fmt.Errorf("unable to write new simulation config in '%s' : %s", csConfig.ConfigPaths.SimulationFilePath, err)
	}

	log.Printf("global simulation: disabled")
	return nil
}

func simulationStatus() error {
	if csConfig.Crowdsec.SimulationConfig == nil {
		log.Printf("global simulation: disabled (configuration file is missing)")
		return nil
	}
	if *csConfig.Crowdsec.SimulationConfig.Simulation {
		log.Println("global simulation: enabled")
		if len(csConfig.Crowdsec.SimulationConfig.Exclusions) > 0 {
			log.Println("Scenarios not in simulation mode :")
			for _, scenario := range csConfig.Crowdsec.SimulationConfig.Exclusions {
				log.Printf("  - %s", scenario)
			}
		}
	} else {
		log.Println("global simulation: disabled")
		if len(csConfig.Crowdsec.SimulationConfig.Exclusions) > 0 {
			log.Println("Scenarios in simulation mode :")
			for _, scenario := range csConfig.Crowdsec.SimulationConfig.Exclusions {
				log.Printf("  - %s", scenario)
			}
		}
	}
	return nil
}

func NewSimulationCmds() *cobra.Command {
	var cmdSimulation = &cobra.Command{
		Use:   "simulation [command]",
		Short: "Manage simulation status of scenarios",
		Example: `cscli simulation status
cscli simulation enable crowdsecurity/ssh-bf
cscli simulation disable crowdsecurity/ssh-bf`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if csConfig.Cscli == nil {
				return fmt.Errorf("you must configure cli before using simulation")
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() != "status" {
				log.Infof("Run 'sudo systemctl reload crowdsec' for the new configuration to be effective.")
			}
		},
	}
	cmdSimulation.Flags().SortFlags = false
	cmdSimulation.PersistentFlags().SortFlags = false

	var forceGlobalSimulation bool
	var cmdSimulationEnable = &cobra.Command{
		Use:     "enable [scenario] [-global]",
		Short:   "Enable the simulation, globally or on specified scenarios",
		Example: `cscli simulation enable`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cwhub.GetHubIdx(csConfig.Cscli); err != nil {
				log.Fatalf("Failed to get Hub index : %v", err)
				log.Infoln("Run 'sudo cscli hub update' to get the hub index")
			}

			if len(args) > 0 {
				for _, scenario := range args {
					var (
						item *cwhub.Item
					)
					item = cwhub.GetItem(cwhub.SCENARIOS, scenario)
					if item == nil {
						log.Errorf("'%s' doesn't exist or is not a scenario", scenario)
						continue
					}
					if !item.Installed {
						log.Warningf("'%s' isn't enabled", scenario)
					}
					isExcluded := inSlice(scenario, csConfig.Crowdsec.SimulationConfig.Exclusions)
					if *csConfig.Crowdsec.SimulationConfig.Simulation && !isExcluded {
						log.Warningf("global simulation is already enabled")
						continue
					}
					if !*csConfig.Crowdsec.SimulationConfig.Simulation && isExcluded {
						log.Warningf("simulation for '%s' already enabled", scenario)
						continue
					}
					if *csConfig.Crowdsec.SimulationConfig.Simulation && isExcluded {
						if err := removeFromExclusion(scenario); err != nil {
							log.Fatalf(err.Error())
						}
						log.Printf("simulation enabled for '%s'", scenario)
						continue
					}
					if err := addToExclusion(scenario); err != nil {
						log.Fatalf(err.Error())
					}
					log.Printf("simulation mode for '%s' enabled", scenario)
				}
				if err := dumpSimulationFile(); err != nil {
					log.Fatalf("simulation enable: %s", err.Error())
				}
			} else if forceGlobalSimulation {
				if err := enableGlobalSimulation(); err != nil {
					log.Fatalf("unable to enable global simulation mode : %s", err.Error())
				}
			} else {
				cmd.Help()
			}
		},
	}
	cmdSimulationEnable.Flags().BoolVarP(&forceGlobalSimulation, "global", "g", false, "Enable global simulation (reverse mode)")
	cmdSimulation.AddCommand(cmdSimulationEnable)

	var cmdSimulationDisable = &cobra.Command{
		Use:     "disable [scenario]",
		Short:   "Disable the simulation mode. Disable only specified scenarios",
		Example: `cscli simulation disable`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				for _, scenario := range args {
					isExcluded := inSlice(scenario, csConfig.Crowdsec.SimulationConfig.Exclusions)
					if !*csConfig.Crowdsec.SimulationConfig.Simulation && !isExcluded {
						log.Warningf("%s isn't in simulation mode", scenario)
						continue
					}
					if !*csConfig.Crowdsec.SimulationConfig.Simulation && isExcluded {
						if err := removeFromExclusion(scenario); err != nil {
							log.Fatalf(err.Error())
						}
						log.Printf("simulation mode for '%s' disabled", scenario)
						continue
					}
					if isExcluded {
						log.Warningf("simulation mode is enabled but is already disable for '%s'", scenario)
						continue
					}
					if err := addToExclusion(scenario); err != nil {
						log.Fatalf(err.Error())
					}
					log.Printf("simulation mode for '%s' disabled", scenario)
				}
				if err := dumpSimulationFile(); err != nil {
					log.Fatalf("simulation disable: %s", err.Error())
				}
			} else if forceGlobalSimulation {
				if err := disableGlobalSimulation(); err != nil {
					log.Fatalf("unable to disable global simulation mode : %s", err.Error())
				}
			} else {
				cmd.Help()
			}
		},
	}
	cmdSimulationDisable.Flags().BoolVarP(&forceGlobalSimulation, "global", "g", false, "Disable global simulation (reverse mode)")
	cmdSimulation.AddCommand(cmdSimulationDisable)

	var cmdSimulationStatus = &cobra.Command{
		Use:     "status",
		Short:   "Show simulation mode status",
		Example: `cscli simulation status`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := simulationStatus(); err != nil {
				log.Fatalf(err.Error())
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
		},
	}
	cmdSimulation.AddCommand(cmdSimulationStatus)

	return cmdSimulation
}
