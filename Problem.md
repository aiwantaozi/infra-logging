## Need to check
1. what the dashboard version, tpr object could seen in the dashboard, but crd can't
2. how to indentify different namespace, we need to distinguish the default(system namespace) and user namespace. the idenfify need to send to k8s controller by UI.
3. When in different namespace, the UI try to get the logging infomation, should return the all namespace info, or just current, is there any place we should return all namespace.
4. Only one fluentd config, for node in different namespace, is fluentd config the same, if same, fluentd may ship infa both to namespace A's elasticsearch and B's elasticsearch


## Solve
1. Gopath type not equal to the vendor path, for example the package apiextensions-apiserver have vendor k8s.io/apimachinery/pkg/apis/meta/v1 and so on, when call the function in apiextensions-apiserver, it try to use the type in apiextensions-apiserver/vendor, not current project vendor, and will face the type different problem. 

```
import(
    extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewLoggingCustomResourceDefinition(group string, labels map[string]string) *extensionsobj.CustomResourceDefinition {
	return &extensionsobj.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: extensionsobj.CustomResourceDefinitionSpec{
			Group:   group,
			Version: loggingv1.Version,
			Scope:   extensionsobj.NamespaceScoped,
			Names: extensionsobj.CustomResourceDefinitionNames{
				Plural: loggingv1.LoggingResourcePlural,
				Kind:   loggingv1.LoggingsKind,
			},
		},
	}
}
```

2. How to deploy the daemonset, it need to communicate with the k8s API, user service account, role, and role binding, it will generate token inside the container path /var/run/secrets/kubernetes.io/serviceaccount/

apiextensions-apiserver try to use the metav1 in itself vendor, will happen can't use k8s.io/apimachinery/pkg/apis/meta/v1 as k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1/vendor/k8s.io/apimachinery/pkg/apis/meta/v1

Solve: delete the the vendor in the apiextensions-apiserver, and it will use the vendor in current project.