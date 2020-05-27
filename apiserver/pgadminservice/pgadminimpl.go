package pgadminservice

/*
Copyright 2020 Crunchy Data Solutions, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"fmt"

	"github.com/crunchydata/postgres-operator/apiserver"
	"github.com/crunchydata/postgres-operator/config"
	"github.com/crunchydata/postgres-operator/internal/pgadmin"
	"github.com/crunchydata/postgres-operator/kubeapi"
	crv1 "github.com/crunchydata/postgres-operator/pkg/apis/crunchydata.com/v1"
	msgs "github.com/crunchydata/postgres-operator/pkg/apiservermsgs"

	log "github.com/sirupsen/logrus"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const pgAdminServiceSuffix = "-pgadmin"

// CreatePgAdmin ...
// pgo create pgadmin mycluster
// pgo create pgadmin --selector=name=mycluster
func CreatePgAdmin(request *msgs.CreatePgAdminRequest, ns, pgouser string) msgs.CreatePgAdminResponse {
	var err error
	resp := msgs.CreatePgAdminResponse{
		Status:  msgs.Status{Code: msgs.Ok},
		Results: []string{},
	}

	log.Debugf("createPgAdmin selector is [%s]", request.Selector)

	// try to get the list of clusters. if there is an error, put it into the
	// status and return
	clusterList, err := getClusterList(request.Namespace, request.Args, request.Selector)
	if err != nil {
		resp.SetError(err.Error())
		return resp
	}

	for _, cluster := range clusterList.Items {
		// check if the current cluster is not upgraded to the deployed
		// Operator version. If not, do not allow the command to complete
		if cluster.Annotations[config.ANNOTATION_IS_UPGRADED] == config.ANNOTATIONS_FALSE {
			resp.Status.Code = msgs.Error
			resp.Status.Msg = cluster.Name + msgs.UpgradeError
			return resp
		}

		log.Debugf("adding pgAdmin to cluster [%s]", cluster.Name)

		// generate the pgtask, starting with spec
		spec := crv1.PgtaskSpec{
			Namespace:   cluster.Namespace,
			Name:        fmt.Sprintf("%s-%s", config.LABEL_PGADMIN_TASK_ADD, cluster.Name),
			TaskType:    crv1.PgtaskPgAdminAdd,
			StorageSpec: cluster.Spec.PrimaryStorage,
			Parameters: map[string]string{
				config.LABEL_PGADMIN_TASK_CLUSTER: cluster.Name,
			},
		}

		task := &crv1.Pgtask{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: spec.Name,
				Labels: map[string]string{
					config.LABEL_PG_CLUSTER:       cluster.Name,
					config.LABEL_PGADMIN_TASK_ADD: "true",
					config.LABEL_PGOUSER:          pgouser,
				},
			},
			Spec: spec,
		}

		if err := kubeapi.Createpgtask(apiserver.RESTClient, task, cluster.Namespace); err != nil {
			log.Error(err)
			resp.SetError("error creating tasks for one or more clusters")
			resp.Results = append(resp.Results, fmt.Sprintf("%s: error - %s", cluster.Name, err.Error()))
			continue
		} else {
			resp.Results = append(resp.Results, fmt.Sprintf("%s pgAdmin addition scheduled", cluster.Name))
		}
	}

	return resp
}

// DeletePgAdmin ...
// pgo delete pgadmin mycluster
// pgo delete pgadmin --selector=name=mycluster
func DeletePgAdmin(request *msgs.DeletePgAdminRequest, ns string) msgs.DeletePgAdminResponse {
	var err error
	resp := msgs.DeletePgAdminResponse{
		Status:  msgs.Status{Code: msgs.Ok},
		Results: []string{},
	}

	log.Debugf("deletePgAdmin selector is [%s]", request.Selector)

	// try to get the list of clusters. if there is an error, put it into the
	// status and return
	clusterList, err := getClusterList(request.Namespace, request.Args, request.Selector)
	if err != nil {
		resp.SetError(err.Error())
		return resp
	}

	for _, cluster := range clusterList.Items {
		// check if the current cluster is not upgraded to the deployed
		// Operator version. If not, do not allow the command to complete
		if cluster.Annotations[config.ANNOTATION_IS_UPGRADED] == config.ANNOTATIONS_FALSE {
			resp.Status.Code = msgs.Error
			resp.Status.Msg = cluster.Name + msgs.UpgradeError
			return resp
		}

		log.Debugf("deleting pgAdmin from cluster [%s]", cluster.Name)

		// generate the pgtask, starting with spec
		spec := crv1.PgtaskSpec{
			Namespace: cluster.Namespace,
			Name:      config.LABEL_PGADMIN_TASK_DELETE + "-" + cluster.Name,
			TaskType:  crv1.PgtaskPgAdminDelete,
			Parameters: map[string]string{
				config.LABEL_PGADMIN_TASK_CLUSTER: cluster.Name,
			},
		}

		task := &crv1.Pgtask{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: spec.Name,
				Labels: map[string]string{
					config.LABEL_PG_CLUSTER:          cluster.Name,
					config.LABEL_PGADMIN_TASK_DELETE: "true",
				},
			},
			Spec: spec,
		}

		if err := kubeapi.Createpgtask(apiserver.RESTClient, task, cluster.Namespace); err != nil {
			log.Error(err)
			resp.SetError("error creating tasks for one or more clusters")
			resp.Results = append(resp.Results, fmt.Sprintf("%s: error - %s", cluster.Name, err.Error()))
			return resp
		} else {
			resp.Results = append(resp.Results, cluster.Name+" pgAdmin delete scheduled")
		}

	}

	return resp
}

// ShowPgAdmin gets information about a PostgreSQL cluster's pgAdmin
// deployment
//
// pgo show pgadmin
// pgo show pgadmin --selector
func ShowPgAdmin(request *msgs.ShowPgAdminRequest, namespace string) msgs.ShowPgAdminResponse {
	log.Debugf("show pgAdmin called, cluster [%v], selector [%s]", request.ClusterNames, request.Selector)

	response := msgs.ShowPgAdminResponse{
		Results: []msgs.ShowPgAdminDetail{},
		Status:  msgs.Status{Code: msgs.Ok},
	}

	// try to get the list of clusters. if there is an error, put it into the
	// status and return
	clusterList, err := getClusterList(request.Namespace, request.ClusterNames, request.Selector)

	if err != nil {
		response.SetError(err.Error())
		return response
	}

	// iterate through the list of clusters to get the relevant pgAdmin
	// information about them
	for _, cluster := range clusterList.Items {
		result := msgs.ShowPgAdminDetail{
			ClusterName: cluster.Spec.Name,
			HasPgAdmin:  true,
		}

		// first, check if the cluster has the pgAdmin label. If it does not, we
		// add it to the list and keep iterating
		clusterLabels := cluster.GetLabels()

		if clusterLabels[config.LABEL_PGADMIN] != "true" {
			result.HasPgAdmin = false
			response.Results = append(response.Results, result)
			continue
		}

		// This takes advantage of pgadmin deployment and pgadmin service
		// sharing a name that is clustername + pgAdminServiceSuffix
		service, _, err := kubeapi.GetService(
			apiserver.Clientset,
			cluster.Name+pgAdminServiceSuffix,
			cluster.Namespace)
		if err != nil {
			response.SetError(err.Error())
			return response
		}

		result.ServiceClusterIP = service.Spec.ClusterIP
		result.ServiceName = service.Name
		if len(service.Spec.ExternalIPs) > 0 {
			result.ServiceExternalIP = service.Spec.ExternalIPs[0]
		}
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			result.ServiceExternalIP = service.Status.LoadBalancer.Ingress[0].IP
		}

		// In the future, construct results to contain individual error stati
		// for now log and return empty content if encountered
		qr, err := pgadmin.GetPgAdminQueryRunner(apiserver.Clientset, apiserver.RESTConfig, &cluster)
		if err != nil {
			log.Error(err)
			continue
		} else if qr != nil {
			names, err := pgadmin.GetUsernames(qr)
			if err != nil {
				log.Error(err)
				continue
			}
			result.Users = names
		}

		// append the result to the list
		response.Results = append(response.Results, result)
	}

	return response
}

// getClusterList tries to return a list of clusters based on either having an
// argument list of cluster names, or a Kubernetes selector
func getClusterList(namespace string, clusterNames []string, selector string) (crv1.PgclusterList, error) {
	clusterList := crv1.PgclusterList{}

	// see if there are any values in the cluster name list or in the selector
	// if nothing exists, return an error
	if len(clusterNames) == 0 && selector == "" {
		err := fmt.Errorf("either a list of cluster names or a selector needs to be supplied for this comment")
		return clusterList, err
	}

	// try to build the cluster list based on either the selector or the list
	// of arguments...or both. First, start with the selector
	if selector != "" {
		err := kubeapi.GetpgclustersBySelector(apiserver.RESTClient, &clusterList,
			selector, namespace)

		// if there is an error, return here with an empty cluster list
		if err != nil {
			return crv1.PgclusterList{}, err
		}
	}

	// now try to get clusters based specific cluster names
	for _, clusterName := range clusterNames {
		cluster := crv1.Pgcluster{}

		_, err := kubeapi.Getpgcluster(apiserver.RESTClient, &cluster,
			clusterName, namespace)

		// if there is an error, capture it here and return here with an empty list
		if err != nil {
			return crv1.PgclusterList{}, err
		}

		// if successful, append to the cluster list
		clusterList.Items = append(clusterList.Items, cluster)
	}

	log.Debugf("clusters founds: [%d]", len(clusterList.Items))

	// if after all this, there are no clusters found, return an error
	if len(clusterList.Items) == 0 {
		err := fmt.Errorf("no clusters found")
		return clusterList, err
	}

	// all set! return the cluster list with error
	return clusterList, nil
}
