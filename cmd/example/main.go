package main

import (
	"fmt"

	"github.com/bytearena/ecs"
)

// The Walk component; always design components as simple containers
// without logic (except for getters/setters if useful)
type Walk struct {
	Direction string
	Distance  float64
}

// The Talk component
type Talk struct {
	Message string
}

func main() {

	// Initialize the ECS manager
	manager := ecs.NewManager()

	// Declare the components
	walk := manager.NewComponent()
	talk := manager.NewComponent()

	// Create 3 entities, and provide their components
	// Component data may be anything (interface{})
	// Use pointers if you want to be able to mutate the data
	manager.NewEntity().
		AddComponent(walk, &Walk{
			Direction: "east",
			Distance:  3.14,
		})

	manager.NewEntity().
		AddComponent(talk, &Talk{
			Message: "Wassup?",
		})

	manager.NewEntity().
		AddComponent(walk, &Walk{
			Direction: "north",
			Distance:  12.4,
		}).
		AddComponent(talk, &Talk{
			Message: "Fluctuat nec mergitur.",
		})

	// Tags are masks that help identify entities that match the required components
	walkers := ecs.BuildTag(walk)
	talkers := ecs.BuildTag(talk)
	walkertalkers := ecs.BuildTag(walkers, talkers)

	// Process the walkers
	fmt.Println("\n# All the walkers walk :")
	for _, result := range manager.Query(walkers) {
		walkAspect := result.Components[walk].(*Walk)
		fmt.Println("I'm walking ", walkAspect.Distance, "km towards", walkAspect.Direction)
	}

	// Process the talkers
	fmt.Println("\n# All the talkers talk (and be mutated) :")
	for _, result := range manager.Query(talkers) {
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println(talkAspect.Message, "Just sayin'.")

		// Here we mutate the component for this entity
		talkAspect.Message = "So I was like 'For real?' and he was like '" + talkAspect.Message + "'"
	}

	// Process the talkers/walkers
	fmt.Println("\n# All the talkers & walkers do their thing :")
	for _, result := range manager.Query(walkertalkers) {
		walkAspect := result.Components[walk].(*Walk)
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println("I'm walking towards", walkAspect.Direction, ";", talkAspect.Message)
	}

	///////////////////////////////////////////////////////////////////////////
	// Demonstrating views
	// To increase speed for repetitive queries, you can create cached views
	// for entities matching a given tag
	///////////////////////////////////////////////////////////////////////////

	fmt.Println("\n# Demonstrating views")

	talkersView := manager.CreateView(talkers)

	//Add a new entity that can talk
	manager.NewEntity().
		AddComponent(talk, &Talk{
			Message: "Ceci n'est pas une pipe",
		})

	fmt.Println("\n# Should print 3 messages :")
	for _, result := range talkersView.Get() {
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println(result.Entity.GetID(), "says", talkAspect.Message)
	}

	manager.DisposeEntities(manager.Query(talkers).Entities()...)

	fmt.Println("\n# Talkers have been disposed; should not print any message below :")
	for _, result := range talkersView.Get() {
		talkAspect := result.Components[talk].(*Talk)
		fmt.Println(result.Entity.GetID(), "says", talkAspect.Message)
	}
}
