package sub

import (
	"fmt"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	necotiatorv1beta1 "github.com/cybozu-go/necotiator/api/v1beta1"
	"github.com/cybozu-go/necotiator/controllers"
	"github.com/cybozu-go/necotiator/hooks"
	"github.com/cybozu-go/necotiator/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func subMain(ns, sa, addr string, port int) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options.zapOpts)))
	logger := ctrl.Log.WithName("setup")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return fmt.Errorf("unable to add client-go objects: %w", err)
	}
	if err := necotiatorv1beta1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("unable to add Necotiator objects: %w", err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      options.metricsAddr,
		HealthProbeBindAddress:  options.probeAddr,
		LeaderElection:          true,
		LeaderElectionID:        options.leaderElectionID,
		LeaderElectionNamespace: ns,
		Host:                    addr,
		Port:                    port,
		CertDir:                 options.certDir,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	ctx := ctrl.SetupSignalHandler()

	if err := (&controllers.TenantResourceQuotaReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(constants.EventRecorderName),
	}).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create Tenant Resource Quota controller: %w", err)
	}

	if err = controllers.SetupMetrics(ctx, mgr.GetClient()); err != nil {
		return fmt.Errorf("unable to setup metrics %w", err)
	}
	if err = hooks.SetupResourceQuotaWebhookWithManager(mgr, ns, sa); err != nil {
		return fmt.Errorf("unable to create ResourceQuota Webhook %w", err)
	}
	if err = hooks.SetupTenantResourceQuotaWebhookWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create TenantResourceQuota webhook %w", err)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %s", err)
	}
	return nil
}
