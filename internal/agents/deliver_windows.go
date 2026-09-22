//go:build windows

package agents

type Delivery struct {
	Via    string `json:"via"`
	Detail string `json:"detail"`
}

func Deliver(pid int, text string) Delivery {
	return Delivery{Via: "copy", Detail: "no delivery adapters on Windows"}
}
