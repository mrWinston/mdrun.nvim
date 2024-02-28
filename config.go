package main

import (
	"encoding/json"
	"fmt"

	"github.com/mrWinston/mdrun.nvim/pkg/runner"
	log "github.com/sirupsen/logrus"
)

//go:generate gomodifytags -file ./config.go -all -add-tags "json,yaml" -transform snakecase -override -w -quiet
type RunnerConfig struct {
	Type      string                 `json:"type" yaml:"type"`
	Languages []string               `json:"languages" yaml:"languages"`
	Image     string                 `json:"image" yaml:"image"`
	Config    runner.CodeblockRunner `json:"config" yaml:"config"`
}

type Config struct {
	StopSignal    string                   `json:"stop_signal" yaml:"stop_signal"`
	DockerRuntime string                   `json:"docker_runtime" yaml:"docker_runtime"`
	RunnerConfigs map[string]*RunnerConfig `json:"runner_configs" yaml:"runner_configs"`
}

func (rc *RunnerConfig) UnmarshalJSON(data []byte) error {
	var rawMap map[string]json.RawMessage

	err := json.Unmarshal(data, &rawMap)
	if err != nil {
		log.Debugf("Error unmarshalling raw: %v", err)
		return err
	}

	typeRaw, ok := rawMap["type"]
	if !ok {
		log.Debugf("Didn't find type: %v", err)
		return fmt.Errorf("Runner config needs 'type' to be set")
	}

	var runnerType string
	err = json.Unmarshal(typeRaw, &runnerType)
	if err != nil {
		log.Debugf("Error parsing runner type: %v", err)
		return fmt.Errorf("Can't parse type into string")
	}
	var languages []string

	langRaw, ok := rawMap["languages"]
	if !ok {
		return fmt.Errorf("Runner config need key 'languages' to be set")
	}

	err = json.Unmarshal(langRaw, &languages)
	if err != nil {
		return fmt.Errorf("Can't parse languages into list of strings")
	}

	var image string
	imageRaw, ok := rawMap["image"]
	if !ok {
		image = ""
	} else {
		err = json.Unmarshal(imageRaw, &image)

		if err != nil {
			image = ""
		}
	}
  

	configRaw, ok := rawMap["config"]
	if !ok {
		return fmt.Errorf("Runner config needs key 'config' to be set")
	}

	var parsedRunner runner.CodeblockRunner

	switch runnerType {
	case "CompiledRunner":
		tmpRunner := &runner.CompiledRunner{}
		err = json.Unmarshal(configRaw, tmpRunner)
		parsedRunner = tmpRunner
	case "InterpretedRunner":
		tmpRunner := &runner.InterpretedRunner{}
		err = json.Unmarshal(configRaw, tmpRunner)
		parsedRunner = tmpRunner
	case "GoRunner":
		tmpRunner := &runner.GoRunner{}
		err = json.Unmarshal(configRaw, tmpRunner)
		parsedRunner = tmpRunner
	case "JavaRunner":
		tmpRunner := &runner.JavaRunner{}
		err = json.Unmarshal(configRaw, tmpRunner)
		parsedRunner = tmpRunner
	case "LuaRunner":
		tmpRunner := &runner.LuaRunner{}
		err = json.Unmarshal(configRaw, tmpRunner)
		parsedRunner = tmpRunner
	case "ShellRunner":
		tmpRunner := &runner.ShellRunner{}
		err = json.Unmarshal(configRaw, tmpRunner)
		parsedRunner = tmpRunner
	default:
		err = fmt.Errorf("Couldn't find Runner Type %s", runnerType)
	}
	if err != nil {
		log.Errorf("Got this error while unmarshalling: %v", err)
		return err
	}

	rc.Type = runnerType
	rc.Languages = languages
	rc.Config = parsedRunner
  rc.Image = image

	return nil
}
