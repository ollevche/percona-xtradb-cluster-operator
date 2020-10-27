package deployment

import (
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	"github.com/percona/percona-xtradb-cluster-operator/pkg/pxc/app"
	"github.com/pkg/errors"
)

func GetBinlogCollectorDeployment(cr *api.PerconaXtraDBCluster) (appsv1.Deployment, error) {
	storage := cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName]
	binlogCollectorName := "bl-collector"
	pxcUser := "operator"
	sleepTime := strconv.FormatInt(cr.Spec.Backup.PITR.TimeBetweenUploads, 10)
	if len(sleepTime) == 0 {
		sleepTime = "60"
	}
	storageEndpoint := strings.TrimPrefix(storage.S3.EndpointURL, "https://")
	labels := map[string]string{
		"app.kubernetes.io/name":       "percona-xtradb-cluster",
		"app.kubernetes.io/instance":   cr.Name,
		"app.kubernetes.io/component":  cr.Name + "-" + binlogCollectorName,
		"app.kubernetes.io/managed-by": "percona-xtradb-cluster-operator",
		"app.kubernetes.io/part-of":    "percona-xtradb-cluster",
	}
	for key, value := range cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].Labels {
		labels[key] = value
	}
	envs := []corev1.EnvVar{
		{
			Name:  "ENDPOINT",
			Value: storageEndpoint,
		},
		{
			Name: "SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: app.SecretKeySelector(storage.S3.CredentialsSecret, "AWS_SECRET_ACCESS_KEY"),
			},
		},
		{
			Name: "ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: app.SecretKeySelector(storage.S3.CredentialsSecret, "AWS_ACCESS_KEY_ID"),
			},
		},
		{
			Name:  "S3_BUCKET",
			Value: storage.S3.Bucket,
		},
		{
			Name:  "PXC_SERVICE",
			Value: cr.Name + "-pxc",
		},
		{
			Name:  "PXC_USER",
			Value: pxcUser,
		},
		{
			Name: "MYSQL_ROOT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: app.SecretKeySelector(cr.Spec.SecretsName, "root"),
			},
		},
		{
			Name: "PXC_PASS",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: app.SecretKeySelector(cr.Spec.SecretsName, pxcUser),
			},
		},
		{
			Name:  "SLEEP_SECONDS",
			Value: sleepTime,
		},
	}
	res, err := app.CreateResources(cr.Spec.Backup.PITR.Resources)
	if err != nil {
		return appsv1.Deployment{}, errors.Wrap(err, "create resources")
	}
	container := corev1.Container{
		Name:            "collector",
		Image:           cr.Spec.Backup.Image,
		ImagePullPolicy: cr.Spec.PXC.ImagePullPolicy,
		Env:             envs,
		SecurityContext: cr.Spec.PXC.ContainerSecurityContext,
		Command:         []string{"binlog-collector"},
		Resources:       res,
	}
	replicas := int32(1)

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-" + binlogCollectorName,
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cr.Name + "-" + binlogCollectorName,
					Namespace: cr.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Containers:         []corev1.Container{container},
					ImagePullSecrets:   cr.Spec.Backup.ImagePullSecrets,
					ServiceAccountName: cr.Spec.Backup.ServiceAccountName,
					SecurityContext:    cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].PodSecurityContext,
					Affinity:           cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].Affinity,
					Tolerations:        cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].Tolerations,
					NodeSelector:       cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].NodeSelector,
					SchedulerName:      cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].SchedulerName,
					PriorityClassName:  cr.Spec.Backup.Storages[cr.Spec.Backup.PITR.StorageName].PriorityClassName,
				},
			},
		},
	}, nil
}
