package utils

import (
	"context"
	"fmt"
	"strings"

	dynatracev1alpha1 "github.com/Dynatrace/dynatrace-oneagent-operator/pkg/apis/dynatrace/v1alpha1"
	"github.com/Dynatrace/dynatrace-oneagent-operator/pkg/dtclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	DynatracePaasToken = "paasToken"
	DynatraceApiToken  = "apiToken"
)

var logger = log.Log.WithName("dynatrace.utils")

// DynatraceClientFunc defines handler func for dynatrace client
type DynatraceClientFunc func(rtc client.Client, instance *dynatracev1alpha1.OneAgent) (dtclient.Client, error)

// BuildDynatraceClient creates a new Dynatrace client using the settings configured on the given instance.
func BuildDynatraceClient(rtc client.Client, instance *dynatracev1alpha1.OneAgent) (dtclient.Client, error) {
	secret := &corev1.Secret{}
	err := rtc.Get(context.TODO(), client.ObjectKey{Namespace: instance.Namespace, Name: GetTokensName(instance)}, secret)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if err = verifySecret(secret); err != nil {
		return nil, err
	}

	// initialize dynatrace client
	var opts []dtclient.Option
	if instance.Spec.SkipCertCheck {
		opts = append(opts, dtclient.SkipCertificateValidation(true))
	}

	p := instance.Spec.Proxy

	if p != nil {
		if p.ValueFrom != "" {
			proxySecret := &corev1.Secret{}
			err := rtc.Get(context.TODO(), client.ObjectKey{Namespace: instance.Namespace, Name: p.ValueFrom}, proxySecret)
			if err != nil {
				logger.Info("Failed to get proxy field within proxy secret!")
			} else {
				proxyURL, err := extractToken(proxySecret, "proxy")
				if err != nil {
					return nil, err
				}
				opts = append(opts, dtclient.Proxy(proxyURL))
			}
		} else if p.Value != "" {
			opts = append(opts, dtclient.Proxy(p.Value))
		}
	}

	apiToken, err := extractToken(secret, DynatraceApiToken)
	if err != nil {
		return nil, err
	}

	paasToken, err := extractToken(secret, DynatracePaasToken)
	if err != nil {
		return nil, err
	}

	dtc, err := dtclient.NewClient(instance.Spec.ApiUrl, apiToken, paasToken, opts...)

	return dtc, err
}

func extractToken(secret *v1.Secret, key string) (string, error) {
	value, ok := secret.Data[key]
	if !ok {
		err := fmt.Errorf("missing token %s", key)
		return "", err
	}

	return strings.TrimSpace(string(value)), nil
}

func verifySecret(secret *v1.Secret) error {
	for _, token := range []string{DynatracePaasToken, DynatraceApiToken} {
		_, err := extractToken(secret, token)
		if err != nil {
			return fmt.Errorf("invalid secret %s, %s", secret.Name, err)
		}
	}

	return nil
}

// StaticDynatraceClient creates a DynatraceClientFunc always returning c.
func StaticDynatraceClient(c dtclient.Client) DynatraceClientFunc {
	return func(_ client.Client, oa *dynatracev1alpha1.OneAgent) (dtclient.Client, error) {
		return c, nil
	}
}

func GetTokensName(oa *dynatracev1alpha1.OneAgent) string {
	secretName := oa.Name
	if oa.Spec.Tokens != "" {
		secretName = oa.Spec.Tokens
	}
	return secretName
}
