package config

import (
	"bytes"
	"encoding/json"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	cmdconfig "k8s.io/kubernetes/pkg/kubectl/cmd/config"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// A part of the general structure of a kubeconfig file
type clusterDetails struct {
	Server string `json:"server"`
}
type clusters struct {
	Cluster clusterDetails `json:"cluster"`
	Name    string         `json:"name"`
}
type contextDetails struct {
	Cluster string `json:"cluster"`
	User    string `json:"user"`
}
type contexts struct {
	Context contextDetails `json:"context"`
	Name    string         `json:"name"`
}
type configView struct {
	Clusters       []clusters `json:"clusters"`
	Contexts       []contexts `json:"contexts"`
	CurrentContext string     `json:"current-context"`
}

// This reads the kubeconfig file by admin context and returns it in json format.
func getConfigView() (string, error) {
	pathOptions := clientcmd.NewDefaultPathOptions()
	streamsIn := &bytes.Buffer{}
	streamsOut := &bytes.Buffer{}
	streamsErrOut := &bytes.Buffer{}
	streams := genericclioptions.IOStreams{
		In:     streamsIn,
		Out:    streamsOut,
		ErrOut: streamsErrOut,
	}
	
	configCmd := cmdconfig.NewCmdConfigView(cmdutil.NewFactory(genericclioptions.NewConfigFlags(false)), streams, pathOptions)
	// "context" is a global flag, inherited from base kubectl command in the real world
	configCmd.Flags().String("context", "kubernetes-admin@kubernetes", "The name of the kubeconfig context to use")
	configCmd.Flags().Parse([]string{"--output=json"})
	if err := configCmd.Execute(); err != nil {
		fmt.Printf("unexpected error executing command: %v", err)
		return "", err
	}

	output := fmt.Sprint(streams.Out)
	return output, nil
}

// GetClusterServerOfCurrentContext provides cluster and server info of the current context
func GetClusterServerOfCurrentContext() (string, string, error) {
	configStr, err := getConfigView()
	if err != nil {
		return "", "", err
	}
	var configViewDet configView
	err = json.Unmarshal([]byte(configStr), &configViewDet)
	if err != nil {
		return "", "", err
	}
	
	currentContext := configViewDet.CurrentContext
	var cluster string
	for _, contextRaw := range configViewDet.Contexts {
		if contextRaw.Name == currentContext {
			cluster = contextRaw.Context.Cluster
		}
	}
	var server string
	for _, clusterRaw := range configViewDet.Clusters {
		if clusterRaw.Name == cluster {
			server = clusterRaw.Cluster.Server
		}
	}
	return cluster, server, nil
}

// GetServerOfCurrentContext provides the server info of the current context
func GetServerOfCurrentContext() (string, error) {
	configStr, err := getConfigView()
	if err != nil {
		return "", err
	}
	var configViewDet configView
	err = json.Unmarshal([]byte(configStr), &configViewDet)
	if err != nil {
		return "", err
	}
	
	currentContext := configViewDet.CurrentContext
	var cluster string
	for _, contextRaw := range configViewDet.Contexts {
		if contextRaw.Name == currentContext {
			cluster = contextRaw.Context.Cluster
		}
	}
	var server string
	for _, clusterRaw := range configViewDet.Clusters {
		if clusterRaw.Name == cluster {
			server = clusterRaw.Cluster.Server
		}
	}
	return server, nil
}