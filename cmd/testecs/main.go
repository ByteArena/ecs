package main

import (
	"fmt"
	"log"

	"github.com/bytearena/ecs"
)

type Walk struct {
	Direction string
	Distance  float64
}

type Talk struct {
	Message string
}

func main() {
	fmt.Println("Hello, ECS !")

	manager := ecs.NewManager()

	walk := manager.NewComponent()
	talk := manager.NewComponent()

	manager.NewEntity().
		AddComponent(walk, &Walk{
			Direction: "north",
			Distance:  12.4,
		}).
		AddComponent(talk, &Talk{
			Message: "Fluctuat nec mergitur.",
		})

	manager.NewEntity().
		AddComponent(walk, &Walk{
			Direction: "east",
			Distance:  3.14,
		})

	manager.NewEntity().
		AddComponent(talk, &Talk{
			Message: "Wassup?",
		})

	walkers := ecs.BuildTag(walk)
	talkers := ecs.BuildTag(talk)
	walkertalkers := ecs.BuildTag(walkers, talkers)

	for _, result := range manager.Query(walkers) {
		walkAspect := result.Components[walk].(*Walk)
		log.Println("I'm walking ", walkAspect.Distance, "km towards", walkAspect.Direction)
	}

	for _, result := range manager.Query(talkers) {
		talkAspect := result.Components[talk].(*Talk)
		log.Println(talkAspect.Message, "Just sayin'.")
		talkAspect.Message = "So I was like 'For real?' and he was like '" + talkAspect.Message + "'"
	}

	for _, result := range manager.Query(walkertalkers) {
		walkAspect := result.Components[walk].(*Walk)
		talkAspect := result.Components[talk].(*Talk)
		log.Println("I'm walking towards", walkAspect.Direction, ";", talkAspect.Message)
	}

	manager.DisposeEntities(manager.Query(walkertalkers).Entities()...)
}
