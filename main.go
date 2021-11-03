package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	kubeconfig string
	namespace  string

	create   int
	update   int
	parallel int
	size     int

	padding = func() (p string) {
		for i := 0; i < size; i++ {
			p += "="
		}
		return
	}()

	rootCmd = &cobra.Command{
		Use:               "ktoolhu",
		Short:             "Ktoolhu is tool to do various weird stuff with Kubernetes API",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	perfLoadConfigMapsCmd = &cobra.Command{
		Use:   "perf-configmaps",
		Short: "Create or update configmaps multiple times to create load",
		Long: `Create or update configmaps multiple times to create load.
Could be useful to check speed of K8s api or trigger etcd compact/defrag feature.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.TODO()

			clientset := initK8s()
			_, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
			if err != nil && !kerrors.IsAlreadyExists(err) {
				fmt.Printf("Failed to create namespace: %+v", err)
				os.Exit(1)
			}

			client := clientset.CoreV1().ConfigMaps(namespace)

			wg := sync.WaitGroup{}

			parallelCh := make(chan struct{}, parallel)

			for i := 0; i < create; i++ {
				parallelCh <- struct{}{}

				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					defer func() { <-parallelCh }()

					cm := buildCM(i, padding)

					_, err := client.Create(ctx, cm, metav1.CreateOptions{})
					if err != nil && !kerrors.IsAlreadyExists(err) {
						fmt.Printf("Failed to create configmap: %+v\n", err)
					}
				}(i)
			}

			wg.Wait()

			for i := 0; i < update; i++ {
				parallelCh <- struct{}{}

				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					defer func() { <-parallelCh }()

					cm := buildCM(i, padding)
					cm.Name = fmt.Sprintf("ktoolhu-%d", i%create)

					_, err := client.Update(ctx, cm, metav1.UpdateOptions{})
					if err != nil {
						fmt.Printf("Failed to create configmap: %+v\n", err)
					}
				}(i)
			}

			wg.Wait()
		},
	}
)

func buildCM(i int, padding string) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{}
	cm.Name = fmt.Sprintf("ktoolhu-%d", i)
	cm.Labels = map[string]string{"app": "ktoolhu"}
	cm.Data = map[string]string{"data": fmt.Sprintf("%d%s", i, padding)}

	return cm
}

func init() {
	if home := homedir.HomeDir(); home != "" {
		rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute patth to the kubeconfig file")
	} else {
		rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}

	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "ktoolhu", "namespace")

	rootCmd.AddCommand(perfLoadConfigMapsCmd)
	perfLoadConfigMapsCmd.Flags().IntVar(&create, "create", 10, "amount cm to create, 0 for unlimited")
	perfLoadConfigMapsCmd.Flags().IntVar(&update, "update", 1000, "amount cm to update, 0 for unlimited")
	perfLoadConfigMapsCmd.Flags().IntVarP(&parallel, "parallel", "p", 1, "amount of parallel threads (aka concurrency)")
	perfLoadConfigMapsCmd.Flags().IntVarP(&size, "size", "s", 1000, "size in bytes")

	for i := 0; i < size; i++ {
		padding += "="
	}
}

func initK8s() *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
