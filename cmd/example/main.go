package main

import (
	"fmt"

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

	// Walkers
	fmt.Println("# All the walkers walk :")
	for _, result := range manager.Query(walkers) {
		walkAspect := result.Components[walk].(*Walk)
		fmt.Println("I'm walking ", walkAspect.Distance, "km towards", walkAspect.Direction)
	}

	fmt.Println("")
	fmt.Println("# All the talkers talk (and be mutated) :")
	for _, result := range manager.Query(talkers) {
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println(talkAspect.Message, "Just sayin'.")
		talkAspect.Message = "So I was like 'For real?' and he was like '" + talkAspect.Message + "'"
	}

	fmt.Println("")
	fmt.Println("# All the talkers & walkers do their thing :")
	for _, result := range manager.Query(walkertalkers) {
		walkAspect := result.Components[walk].(*Walk)
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println("I'm walking towards", walkAspect.Direction, ";", talkAspect.Message)
	}

	///////////////////////////////////////////////////////////////////////////
	// Demonstrating views
	///////////////////////////////////////////////////////////////////////////

	fmt.Println("")
	fmt.Println("# Demonstrating views")

	talkersView := manager.CreateView(talkers)

	manager.NewEntity().
		AddComponent(talk, &Talk{
			Message: "Ceci n'est pas une pipe",
		})

	fmt.Println("")
	fmt.Println("# Should print 2 messages :")
	for _, result := range talkersView.Get() {
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println(result.Entity.GetID(), "says", talkAspect.Message)
	}

	manager.DisposeEntities(manager.Query(talkers).Entities()...)

	fmt.Println("")
	fmt.Println("# Talkers have been disposed; should not print any message below :")
	for _, result := range talkersView.Get() {
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println(result.Entity.GetID(), "says", talkAspect.Message)
	}
}
