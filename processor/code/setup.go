package code

import (
	"errors"
	"fmt"

	"github.com/jcelliott/lumber"
	"github.com/nanobox-io/nanobox-golang-stylish"

	"github.com/nanobox-io/golang-docker-client"
	"github.com/nanobox-io/nanobox/models"
	"github.com/nanobox-io/nanobox/processor"
	"github.com/nanobox-io/nanobox/provider"
	"github.com/nanobox-io/nanobox/util"
	"github.com/nanobox-io/nanobox/util/data"
	"github.com/nanobox-io/nanobox/util/ip_control"
	"github.com/nanobox-io/nanobox/util/print"
)

type codeSetup struct {
	config processor.ProcessConfig
	fail   bool
}

var missingImageOrName = errors.New("missing image or name")

func init() {
	processor.Register("code_setup", codeSetupFunc)
}

func codeSetupFunc(config processor.ProcessConfig) (processor.Processor, error) {
	// confirm the provider is an accessable one that we support.

	return &codeSetup{config: config}, nil
}

func (self *codeSetup) clean(fn func()) func() {
	return func() {
		if self.fail {
			fn()
		}
	}
}

func (self codeSetup) Results() processor.ProcessConfig {
	return self.config
}

func (self *codeSetup) Process() error {
	// make sure i was given a name and image
	if self.config.Meta["name"] == "" || self.config.Meta["image"] == "" {
		return missingImageOrName
	}

	// get the service from the database
	service := models.Service{}
	err := data.Get(util.AppName(), self.config.Meta["name"], &service)
	if err == nil {
		// quit early if the service was found to be created already
		return nil
	}

	prefix := fmt.Sprintf("%s+ Pulling %s -", stylish.GenerateNestedPrefix(self.config.DisplayLevel), self.config.Meta["image"])
	_, err = docker.ImagePull(self.config.Meta["image"], &print.DockerPercentDisplay{Prefix: prefix})
	if err != nil {
		return err
	}

	local_ip, err := ip_control.ReserveLocal()
	if err != nil {
		return err
	}
	defer self.clean(func() {
		ip_control.ReturnIP(local_ip)
	})()

	global_ip, err := ip_control.ReserveGlobal()
	if err != nil {
		self.fail = true
		return err
	}
	defer self.clean(func() {
		ip_control.ReturnIP(global_ip)
	})()

	fmt.Println("-> building container", self.config.Meta["name"])
	config := docker.ContainerConfig{
		Name:    fmt.Sprintf("%s-%s", util.AppName(), self.config.Meta["name"]),
		Image:   self.config.Meta["image"],
		Network: "virt",
		IP:      local_ip.String(),
	}

	// create docker container
	container, err := docker.CreateContainer(config)
	if err != nil {
		self.fail = true
		lumber.Error("container: ", err)
		return err
	}
	defer self.clean(func() {
		docker.ContainerRemove(container.ID)
	})()

	err = provider.AddIP(global_ip.String())
	if err != nil {
		self.fail = true
		lumber.Error("addip: ", err)
		return err
	}
	defer self.clean(func() {
		provider.RemoveIP(global_ip.String())
	})()

	err = provider.AddNat(global_ip.String(), local_ip.String())
	if err != nil {
		self.fail = true
		lumber.Error("addnat: ", err)
		return err
	}
	defer self.clean(func() {
		provider.RemoveNat(global_ip.String(), local_ip.String())
	})()

	// save service in DB
	service.ID = container.ID
	service.Name = self.config.Meta["name"]
	service.ExternalIP = global_ip.String()
	service.InternalIP = local_ip.String()

	// save the service
	err = data.Put(util.AppName(), self.config.Meta["name"], service)
	if err != nil {
		self.fail = true
		lumber.Error("insert data: ", err)
		return err
	}
	return nil
}
