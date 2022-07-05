package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	appName = "ktoolhu"
)

var (
	kubeconfig string
	namespace  string

	create   int
	update   int
	parallel int
	size     int

	padding string

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

			runParallel(parallel, create, func(i int) {
				cm := buildCM(i, padding)

				_, err := client.Create(ctx, cm, metav1.CreateOptions{})
				if err != nil && !kerrors.IsAlreadyExists(err) {
					fmt.Printf("Failed to create configmap: %+v\n", err)
				}
			})

			runParallel(parallel, update, func(i int) {
				cm := buildCM(i, padding)
				cm.Name = fmt.Sprintf("ktoolhu-%d", i%create)

				_, err := client.Update(ctx, cm, metav1.UpdateOptions{})
				if err != nil {
					fmt.Printf("Failed to create configmap: %+v\n", err)
				}
			})
		},
	}

	restartAllCmd = &cobra.Command{
		Use:   "restart-all",
		Short: "Restart all workload in cluster or namespace",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			clientset := initK8s()

			var namespaces []corev1.Namespace

			if namespace != "" {
				ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					fmt.Printf("Failed to get namespace %s: %+v\n", namespace, err)
					os.Exit(1)
				}

				namespaces = []corev1.Namespace{*ns}
			} else {
				namespacesList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
				if err != nil {
					fmt.Printf("Failed to list namespaces: %+v\n", err)
					os.Exit(1)
				}
				namespaces = namespacesList.Items
			}

			for _, ns := range namespaces {
				deployments, err := clientset.AppsV1().Deployments(ns.Name).List(ctx, metav1.ListOptions{})
				if err != nil {
					fmt.Printf("Failed to list deployments: %+v\n", err)
					os.Exit(1)
				}

				for _, deployment := range deployments.Items {
					fmt.Printf("Namespace %s, restarting deployment %s\n", ns.Name, deployment.Name)
					patch, err := createRestartPatch(&deployment)
					if err != nil {
						fmt.Printf("Failed to create patch to restart %s: %+v\n", deployment.Name, err)
						os.Exit(1)
					}
					_, err = clientset.AppsV1().Deployments(ns.Name).Patch(ctx, deployment.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
					if err != nil {
						fmt.Printf("Failed to patch : %+v\n", err)
						os.Exit(1)
					}
				}

				daemonsets, err := clientset.AppsV1().DaemonSets(ns.Name).List(ctx, metav1.ListOptions{})
				if err != nil {
					fmt.Printf("Failed to list deployments: %+v\n", err)
					os.Exit(1)
				}

				for _, daemonset := range daemonsets.Items {
					fmt.Printf("Namespace %s, restarting daemonset %s\n", ns.Name, daemonset.Name)
					patch, err := createRestartPatch(&daemonset)
					if err != nil {
						fmt.Printf("Failed to create patch to restart %s: %+v\n", daemonset.Name, err)
						os.Exit(1)
					}
					_, err = clientset.AppsV1().DaemonSets(ns.Name).Patch(ctx, daemonset.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
					if err != nil {
						fmt.Printf("Failed to patch: %+v\n", err)
						os.Exit(1)
					}
				}

				statefulsets, err := clientset.AppsV1().StatefulSets(ns.Name).List(ctx, metav1.ListOptions{})
				if err != nil {
					fmt.Printf("Failed to list statefulsets: %+v\n", err)
					os.Exit(1)
				}

				for _, statefulset := range statefulsets.Items {
					fmt.Printf("Namespace %s, restarting statefulset %s\n", ns.Name, statefulset.Name)
					patch, err := createRestartPatch(&statefulset)
					if err != nil {
						fmt.Printf("Failed to create patch to restart %s: %+v\n", statefulset.Name, err)
						os.Exit(1)
					}
					_, err = clientset.AppsV1().StatefulSets(ns.Name).Patch(ctx, statefulset.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
					if err != nil {
						fmt.Printf("Failed to patch: %+v\n", err)
						os.Exit(1)
					}
				}
			}

		},
	}

	secretCmd = &cobra.Command{
		Use:   "secret",
		Short: "Encode or decode k8s secret from stdin to stdout (yaml or json)",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stderr, "Reading from stdin...")
			inputBytes, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				fmt.Println("Failed to read from stdin:", err.Error())
				os.Exit(1)
			}

			secret := map[string]interface{}{}
			if err = yaml.Unmarshal(inputBytes, secret); err != nil {
				if err2 := json.Unmarshal(inputBytes, &secret); err2 != nil {
					fmt.Println("Failed to parse input as yaml or json:", err, err2)
					os.Exit(1)
				}
			}

			data, ok := secret["data"].(map[interface{}]interface{})
			if !ok {
				fmt.Println("Input does not look like secret (missing 'data' or not map type)")
				os.Exit(1)
			}

			encode := false
			for k, v := range data {
				vv, ok := v.(string)
				if !ok {
					fmt.Println("Key", k, "has non string type")
					os.Exit(1)
				}

				if !encode {
					if decoded, err := base64.StdEncoding.DecodeString(vv); err == nil {
						data[k] = string(decoded)
						continue
					}
					encode = true
				}
				if encode {
					data[k] = base64.StdEncoding.EncodeToString([]byte(vv))
				}
			}

			secret["data"] = data

			decoded, err := yaml.Marshal(secret)
			if err != nil {
				fmt.Println("Failed to marshal to yaml:", err.Error())
				os.Exit(1)
			}

			fmt.Println(string(decoded))
		},
	}
)

func createRestartPatch(obj runtime.Object) ([]byte, error) {
	obj2 := obj.DeepCopyObject()
	obj2Unstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj2)
	if err != nil {
		return nil, err
	}

	metadata, ok := obj2Unstructured["spec"].(map[string]interface{})["template"].(map[string]interface{})["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Failed to extrace template metadata")
	}

	if _, ok := metadata["annotations"]; !ok {
		metadata["annotations"] = map[string]interface{}{}
	}

	annotations := metadata["annotations"].(map[string]interface{})

	annotations[appName+"/restartedAt"] = time.Now().Format(time.RFC3339)

	objBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	obj2Bytes, err := json.Marshal(obj2Unstructured)
	if err != nil {
		return nil, err
	}

	return strategicpatch.CreateTwoWayMergePatch(objBytes, obj2Bytes, obj)
}

func buildCM(i int, padding string) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{}
	cm.Name = fmt.Sprintf("ktoolhu-%d", i)
	cm.Labels = map[string]string{"app": "ktoolhu"}
	cm.Data = map[string]string{"data": fmt.Sprintf("%d%s", i, padding)}

	return cm
}

func runParallel(parallel int, count int, f func(int)) {
	parallelCh := make(chan struct{}, parallel)

	wg := sync.WaitGroup{}

	for i := 0; i < count; i++ {
		parallelCh <- struct{}{}

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-parallelCh }()

			f(i)
		}(i)
	}

	wg.Wait()
}

func init() {
	if home := homedir.HomeDir(); home != "" {
		rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "absolute path to the kubeconfig file")
	} else {
		rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}

	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "ktoolhu", "namespace")

	rootCmd.AddCommand(perfLoadConfigMapsCmd)
	perfLoadConfigMapsCmd.Flags().IntVar(&create, "create", 10, "amount cm to create, 0 for unlimited")
	perfLoadConfigMapsCmd.Flags().IntVar(&update, "update", 1000, "amount cm to update, 0 for unlimited")
	perfLoadConfigMapsCmd.Flags().IntVarP(&parallel, "parallel", "p", 1, "amount of parallel threads (aka concurrency)")
	perfLoadConfigMapsCmd.Flags().IntVarP(&size, "size", "s", 1000, "size in bytes")

	rootCmd.AddCommand(restartAllCmd)
	rootCmd.AddCommand(secretCmd)

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
