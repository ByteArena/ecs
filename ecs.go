package ecs

import (
	"strconv"
	"sync"
	"sync/atomic"
)

type EntityID uint32

func (id EntityID) String() string {
	return strconv.Itoa(int(id))
}

type ComponentID uint32

type Tag uint64 // limited to 64 components

type Manager struct {
	lock            *sync.RWMutex
	entityIdInc     uint32
	componentNumInc uint32 // limited to 64

	entities     []*Entity
	entitiesByID map[EntityID]*Entity
	components   []*Component
}

type Component struct {
	id         ComponentID
	tag        Tag
	datalock   *sync.RWMutex
	data       map[EntityID]interface{}
	destructor func(entity *Entity, data interface{})
}

func (component *Component) SetDestructor(destructor func(entity *Entity, data interface{})) {
	component.destructor = destructor
}

func (component Component) GetID() ComponentID {
	return component.id
}

type Entity struct {
	ID         EntityID
	components Tag
}

func (entity *Entity) GetID() EntityID {
	return entity.ID
}

func NewManager() *Manager {
	return &Manager{
		entityIdInc:     0,
		componentNumInc: 0,
		entitiesByID:    make(map[EntityID]*Entity),
		lock:            &sync.RWMutex{},
	}
}

func BuildTag(elements ...interface{}) Tag {

	tag := Tag(0)

	for _, element := range elements {
		if component, ok := element.(*Component); ok {
			tag |= component.tag
		} else if othertag, ok := element.(Tag); ok {
			tag |= othertag
		} else {
			panic("Invalid type passed to BuildTag; accepts only <*Component> and <Tag> types.")
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

	manager.lock.Lock()
	manager.entities = append(manager.entities, entity)
	manager.entitiesByID[entity.ID] = entity
	manager.lock.Unlock()

	return entity
}

func (manager *Manager) NewComponent() *Component {

	if manager.componentNumInc >= 63 {
		panic("Component overflow (limited to 64)")
	}

	nextid := ComponentID(atomic.AddUint32(&manager.componentNumInc, 1))
	id := nextid - 1 // to start at 0

	component := &Component{
		id:       id,
		tag:      (1 << id), // set bit on position corresponding to component number
		data:     make(map[EntityID]interface{}),
		datalock: &sync.RWMutex{},
	}

	manager.lock.Lock()
	manager.components = append(manager.components, component)
	manager.lock.Unlock()

	return component
}

func (manager Manager) GetEntityByID(id EntityID, tag Tag) *QueryResult {

	manager.lock.RLock()
	res, ok := manager.entitiesByID[id]

	if !ok {
		manager.lock.RUnlock()
		return nil
	}

	components := manager.fetchComponentsForEntity(res, tag)
	manager.lock.RUnlock()

	if components == nil {
		return nil
	}

	return &QueryResult{
		Entity:     res,
		Components: components,
	}

}

func (entity *Entity) AddComponent(component *Component, componentdata interface{}) *Entity {
	component.datalock.Lock()
	component.data[entity.ID] = componentdata
	component.datalock.Unlock()
	entity.components |= component.tag
	return entity
}

func (entity *Entity) RemoveComponent(component *Component) *Entity {
	if component.destructor != nil {
		if data, ok := component.data[entity.ID]; ok {
			component.destructor(entity, data)
		}
	}

	component.datalock.Lock()
	delete(component.data, entity.ID)
	component.datalock.Unlock()

	entity.components ^= component.tag
	return entity
}

func (entity Entity) HasComponent(component *Component) bool {
	return entity.components&component.tag != 0x0000
}

func (entity Entity) GetComponentData(component *Component) (interface{}, bool) {
	component.datalock.RLock()
	data, ok := component.data[entity.ID]
	component.datalock.RUnlock()

	return data, ok
}

func (manager *Manager) DisposeEntities(entities ...*Entity) {
	for _, entity := range entities {
		manager.DisposeEntity(entity)
	}
}

func (manager *Manager) DisposeEntity(entity *Entity) {
	manager.lock.Lock()
	for _, component := range manager.components {
		if entity.HasComponent(component) {
			entity.RemoveComponent(component)
		}
	}
	manager.entitiesByID[entity.ID] = nil
	manager.lock.Unlock()
}

type QueryResult struct {
	Entity     *Entity
	Components map[ComponentID]interface{}
}

func (manager *Manager) fetchComponentsForEntity(entity *Entity, tag Tag) map[ComponentID]interface{} {

	if entity.components&tag != tag {
		return nil
	}

	componentMap := make(map[ComponentID]interface{})

	for _, component := range manager.components {
		if component.tag&tag == component.tag {
			data, ok := entity.GetComponentData(component)
			if !ok {
				return nil // if one of the required components is not set, return nothing !
			}

			componentMap[component.id] = data
		}

		// fmt.Printf("-------------\n")
		// fmt.Printf("%16b : %s\n", int64(tag), "tag")
		// fmt.Printf("%16b : %s\n", int64(component.tag), "component.tag")
		// fmt.Printf("%16b : %s\n", int64(entity.components), "entity.tag")
		// fmt.Printf("//////////////////\n")
	}

	return componentMap
}

func (manager *Manager) Query(tag Tag) []QueryResult {

	matches := make([]QueryResult, 0)

	manager.lock.RLock()
	for _, entity := range manager.entities {
		if entity.components&tag == tag {

			componentMap := make(map[ComponentID]interface{})

			for _, component := range manager.components {
				if component.tag&tag == component.tag {
					data, _ := entity.GetComponentData(component)
					componentMap[component.id] = data
				}
			}

			matches = append(matches, QueryResult{
				Entity:     entity,
				Components: componentMap,
			})

		}
	}
	manager.lock.RUnlock()

	return matches
}
