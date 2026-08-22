package main

import "example/sensorHub/cmd"

var version = "dev"

func main() {
	cmd.Execute(version)
}
