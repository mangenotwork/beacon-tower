package main

import (
	"beacon-tower/udp"
)

func main() {
	udp.NewServers("0.0.0.0", 12345)
}
