package operator

import (
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	"github.com/kyma-incubator/hydroform/types"
	gardener "github.com/kyma-incubator/terraform-provider-gardener/provider"
	"github.com/pkg/errors"

	"github.com/hashicorp/terraform/terraform"
	terraformClient "github.com/kyma-incubator/hydroform/internal/terraform"
	"github.com/terraform-providers/terraform-provider-google/google"
)

const (
	awsClusterTemplate = ``

	azureClusterTemplate = ``

	gcpClusterTemplate = `
  variable "node_count"    		{}
  variable "cluster_name"  		{}
  variable "credentials_file_path" 	{}
  variable "project"       		{}
  variable "location"      		{}
  variable "machine_type"  		{}
  variable "kubernetes_version"   	{}
  variable "disk_size" 			{}

  provider "google" {
    	credentials   = "${file("${var.credentials_file_path}")}"
	project       = "${var.project}"
  }

  resource "google_container_cluster" "gke_cluster" {
    	name               = "${var.cluster_name}"
    	location 	   = "${var.location}"
    	initial_node_count = "${var.node_count}"
    	min_master_version = "${var.kubernetes_version}"
    	node_version       = "${var.kubernetes_version}"
    
    node_config {
      	machine_type = "${var.machine_type}"
		disk_size_gb = "${var.disk_size}"
    }

    maintenance_policy {
      	daily_maintenance_window {
        	start_time = "03:00"
      		}
    	}
  }

  output "endpoint" {
    value = "${google_container_cluster.gke_cluster.endpoint}"
  }

  output "cluster_ca_certificate" {
    value = "${google_container_cluster.gke_cluster.master_auth.0.cluster_ca_certificate}"
  }
`

	gardenerClusterTemplate = `
variable "target_provider"			{}
variable "target_profile"			{}
variable "target_seed"			    {}
variable "target_secret"			{}
variable "node_count"    			{}
variable "cluster_name"  			{}
variable "credentials_file_path" 	{}
variable "namespace"       			{}
variable "location"      			{}
variable "zone"      				{}
variable "workercidr"      			{}
variable "vnetcidr"					{}
variable "vpccidr" 					{}
variable "publicscidr" 				{}
variable "internalscidr" 			{}
variable "machine_type"  			{}
variable "kubernetes_version"   	{}
variable "disk_size" 				{}
variable "disk_type" 				{}
variable "autoscaler_min" 			{}
variable "autoscaler_max" 			{}
variable "max_surge" 				{}
variable "max_unavailable" 			{}

provider "gardener" {
	kube_file          = "${file("${var.credentials_file_path}")}"
}

resource "gardener_shoot" "test_cluster" {
	metadata {
	  name      = "${var.cluster_name}"
	  namespace = "${var.namespace}"
  
	}
  
	spec {
	  cloud {
		profile = "${var.target_profile}"
		region  = "${var.location}"
		seed    = "${var.target_seed}"
		secret_binding_ref {
		  name = "${var.target_secret}"
		}

		{{ if eq (index . "target_provider") "gcp" }}
		gcp {  
          networks {
			workers = ["${var.workercidr}"]
		  }
		{{ end }}

		{{ if eq (index . "target_provider") "azure" }}
		azure {  
          networks {
			vnet    = [{cidr = "${var.vnetcidr}"}]
			workers = "${var.workercidr}"
		  }
		{{ end }}

		{{ if eq (index . "target_provider") "aws" }}
		aws {  
          networks {
			workers       = ["${var.workercidr}"]
			public		  = ["${var.publicscidr}"]
			internal	  = ["${var.internalscidr}"]
			vpc			  = [{cidr = "${var.vpccidr}"}]
		  }
		{{ end }}

		  {{range (seq (index . "node_count"))}}
		  worker {
			  name            = "cpu-worker-{{.}}"
			  machine_type    = "${var.machine_type}"
			  auto_scaler_min = "${var.autoscaler_min}"
			  auto_scaler_max = "${var.autoscaler_max}"
			  max_surge       = "${var.max_surge}"
			  max_unavailable = "${var.max_unavailable}"
			  volume_size     = "${var.disk_size}Gi"
			  volume_type     = "${var.disk_type}"
		  }
		  {{end}}
          {{ if not (eq (index . "target_provider") "azure") }}
		  zones = ["${var.zone}"]
          {{ end }}
		}
	  }
  
	  kubernetes {
		version = "${var.kubernetes_version}"
	  }
	}
  }
`
)

// Terraform is an Operator.
type Terraform struct {
}

// Create creates a new cluster for a specific provider based on configuration details. It returns a ClusterInfo object with provider-related information, or an error if cluster provisioning failed.
func (t *Terraform) Create(providerType types.ProviderType, configuration map[string]interface{}) (*types.ClusterInfo, error) {
	platform, err := t.newPlatform(providerType, configuration)
	if err != nil {
		return nil, err
	}

	state, err := platform.Apply(terraformClient.NewState(), false)
	if err != nil {
		return &types.ClusterInfo{
			InternalState: &types.InternalState{TerraformState: state},
			Status:        &types.ClusterStatus{Phase: types.Errored},
		}, errors.Wrap(err, "unable to provision cluster")
	}

	var certificateData []byte
	var endpoint string
	if len(state.Modules) > 0 {
		if val, ok := state.Modules[0].Outputs["cluster_ca_certificate"]; ok {
			certificateData, err = base64.StdEncoding.DecodeString(fmt.Sprintf("%v", val.Value))
			if err != nil {
				return &types.ClusterInfo{
					InternalState: &types.InternalState{TerraformState: state},
					Status:        &types.ClusterStatus{Phase: types.Errored},
				}, errors.Wrap(err, "Unable to decode certificate data")
			}
		}
		if val, ok := state.Modules[0].Outputs["endpoint"]; ok {
			endpoint = fmt.Sprintf("%v", val.Value)
		}
	}

	return &types.ClusterInfo{
		Endpoint:                 endpoint,
		CertificateAuthorityData: certificateData,
		InternalState:            &types.InternalState{TerraformState: state},
		Status:                   &types.ClusterStatus{Phase: types.Provisioned},
	}, nil
}

// Delete removes an existing cluster or returns an error if removing the cluster is not possible.
func (t *Terraform) Delete(state *types.InternalState, providerType types.ProviderType, configuration map[string]interface{}) error {
	platform, err := t.newPlatform(providerType, configuration)
	if err != nil {
		return err
	}

	_, err = platform.Apply(state.TerraformState, true)
	return errors.Wrap(err, "unable to deprovision cluster")
}

func (t *Terraform) newPlatform(providerType types.ProviderType, configuration map[string]interface{}) (*terraformClient.Platform, error) {
	var resourceProvider terraform.ResourceProvider
	var clusterTemplate string
	//providerName must match the name of the provider on the templates
	var providerName string

	switch providerType {
	case types.GCP:
		resourceProvider = google.Provider()
		clusterTemplate = gcpClusterTemplate
		providerName = "google"
	case types.AWS:
		//resourceProvider = aws.Provider()
		//clusterTemplate = awsClusterTemplate
		//providerName = "aws"
		return nil, errors.New("aws not supported yet")
	case types.Azure:
		//resourceProvider = azure.Provider()
		//clusterTemplate = azureClusterTemplate
		//providerName = "azure"
		return nil, errors.New("azure not supported yet")
	case types.Gardener:
		resourceProvider = gardener.Provider()
		providerName = "gardener"

		expTemplate, err := expandGardenerClusterTemplate(configuration)
		if err != nil {
			return nil, err
		}
		clusterTemplate = expTemplate
	default:
		return nil, errors.New("unknown provider")
	}

	platform := terraformClient.NewPlatform(clusterTemplate)
	platform.AddProvider(providerName, resourceProvider)
	for k, v := range configuration {
		platform.Var(k, v)
	}

	return platform, nil
}

func expandGardenerClusterTemplate(config map[string]interface{}) (string, error) {

	funcs := template.FuncMap{
		"seq": func(n int) []int {
			r := make([]int, n)

			for i := 0; i < n; i++ {
				r[i] = i
			}
			return r
		},
	}

	t := template.Must(template.New("gardenerCluster").Funcs(funcs).Parse(gardenerClusterTemplate))
	s := &strings.Builder{}
	if err := t.Execute(s, config); err != nil {
		return "", err
	}
	return s.String(), nil
}
