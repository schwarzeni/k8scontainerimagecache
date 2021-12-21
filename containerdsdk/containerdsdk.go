package main

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/namespaces"

	"github.com/containerd/containerd"
)

func main() {
	context := context.Background()
	// create a context for docker
	context = namespaces.WithNamespace(context, "docker")
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	images, err := client.ImageService().List(context)
	if err != nil {
		panic(err)
	}
	for _, image := range images {
		fmt.Println("delete image:", image.Name)
		if err := client.ImageService().Delete(context, image.Name); err != nil {
			panic(err)
		}
	}
	//image, err := client.Pull(context, "docker.io/library/nginx:alpine")
	image, err := client.Pull(context, "127.0.0.1:8888/library/nginx:alpine")
	if err != nil {
		panic(err)
	}
	fmt.Println(image)
	if err := client.ImageService().Delete(context, image.Name()); err != nil {
		panic(err)
	}
}
