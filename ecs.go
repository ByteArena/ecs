package ecs

import (
	"sync/atomic"
)

type EntityID uint32
type ComponentID uint32

type Tag uint64 // limited to 64 components

type Manager struct {
	entityIdInc     uint32
	componentNumInc uint32 // limited to 64

	entities   []*Entity
	components []*Component
}

type Component struct {
	id   ComponentID
	tag  Tag
	data map[EntityID]interface{}
}

type Entity struct {
	ID         EntityID
	components Tag
}

func NewManager() *Manager {
	return &Manager{
		entityIdInc:     0,
		componentNumInc: 0,
	}
}

func ComposeSignature(elements ...interface{}) Tag {

	tag := Tag(0)

	for _, element := range elements {
		if component, ok := element.(*Component); ok {
			tag |= component.tag
		} else if othertag, ok := element.(Tag); ok {
			tag |= othertag
		} else {
			panic("Invalid type passed to Composesignature; accepts only <*Component> and <Tag> types.")
		}
	}

	return tag
}

func (manager *Manager) NewEntity() *Entity {

	nextid := ComponentID(atomic.AddUint32(&manager.componentNumInc, 1))
	id := nextid - 1 // to start at 0

	entity := &Entity{
		ID: EntityID(id),
	}

	manager.entities = append(manager.entities, entity)

	return entity
}

func (manager *Manager) NewComponent() *Component {

	if manager.componentNumInc >= 63 {
		panic("Component overflow (limited to 64)")
	}

	nextid := ComponentID(atomic.AddUint32(&manager.componentNumInc, 1))
	id := nextid - 1 // to start at 0

	component := &Component{
		id:   id,
		tag:  (1 << id), // set bit on position corresponding to component number
		data: make(map[EntityID]interface{}),
	}

	manager.components = append(manager.components, component)

	return component
}

func (entity *Entity) AddComponent(component *Component, componentdata interface{}) *Entity {
	component.data[entity.ID] = componentdata
	entity.components |= component.tag
	return entity
}

func (entity *Entity) RemoveComponent(component *Component) *Entity {
	delete(component.data, entity.ID)
	entity.components ^= component.tag
	return entity
}

func (entity Entity) HasComponent(component *Component) bool {
	return entity.components&component.tag != 0x0000
}

func (entity Entity) GetComponentData(component *Component) interface{} {
	if data, ok := component.data[entity.ID]; ok {
		return data
	}

	return nil
}

func (manager *Manager) DisposeEntities(entities ...*Entity) {
	for _, entity := range entities {
		manager.DisposeEntity(entity)
	}
}

func (manager *Manager) DisposeEntity(entity *Entity) {
	for _, component := range manager.components {
		if entity.HasComponent(component) {
			entity.RemoveComponent(component)
		}
	}
}

func (manager *Manager) Query(tag Tag) []*Entity {

	matches := make([]*Entity, 0)
	for _, entity := range manager.entities {
		if entity.components&tag == tag {
			matches = append(matches, entity)
		}
	}

	return matches
}
