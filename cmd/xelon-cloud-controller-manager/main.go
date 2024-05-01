package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"

	"github.com/Xelon-AG/xelon-cloud-controller-manager/internal/xelon"
)

func main() {
	rand.NewSource(time.Now().UnixNano())

	opts, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Errorf("failed to initialize command options: %v", err)
		os.Exit(1)
	}
	opts.KubeCloudShared.CloudProvider.Name = xelon.ProviderName
	opts.Authentication.SkipInClusterLookup = true

	command := app.NewCloudControllerManagerCommand(
		opts,
		cloudInitializer,
		app.DefaultInitFuncConstructors,
		map[string]string{},
		flag.NamedFlagSets{},
		wait.NeverStop,
	)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cloudInitializer(c *config.CompletedConfig) cloudprovider.Interface {
	cloudConfig := c.ComponentConfig.KubeCloudShared.CloudProvider
	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Errorf("failed to initizlize cloud provider: %v", err)
		os.Exit(1)
	}
	if cloud == nil {
		klog.Error("cloud provider is nil")
		os.Exit(1)
	}
	return cloud
}
