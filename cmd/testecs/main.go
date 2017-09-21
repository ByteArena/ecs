package main

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"

	"github.com/bytearena/ecs"
)

func main() {
	fmt.Println("Hello, ECS !")

	manager := ecs.NewManager()

	walk := manager.NewComponent()
	talk := manager.NewComponent()

	manager.NewEntity().
		AddComponent(walk, 5).
		AddComponent(talk, "wassup")

	manager.NewEntity().
		AddComponent(walk, 1)

	manager.NewEntity().
		AddComponent(talk, "I'm just a talker")

	walkers := ecs.ComposeSignature(walk)
	talkers := ecs.ComposeSignature(talk)
	walkertalkers := ecs.ComposeSignature(walkers, talkers)

	spew.Dump("walkers", manager.Query(walkers))
	spew.Dump("talkers", manager.Query(talkers))
	spew.Dump("walkerstalkers", manager.Query(walkertalkers))

	manager.DisposeEntities(manager.Query(walkertalkers)...)

	spew.Dump("walkerstalkers", manager.Query(walkertalkers))
}
