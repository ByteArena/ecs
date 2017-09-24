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

func (tag Tag) isIncludedIn(biggertag Tag) bool {
	return tag&biggertag == tag
}

func (tag Tag) includes(smallertag Tag) bool {
	return tag&smallertag == smallertag
}

type View struct {
	tag      Tag
	entities []*QueryResult
	lock     *sync.RWMutex
}

func (v View) Get() []*QueryResult {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.entities
}

func (v *View) add(entity *Entity) {
	v.lock.Lock()
	v.entities = append(v.entities, entity.manager.GetEntityByID(
		entity.ID,
		v.tag,
	))
	v.lock.Unlock()
}

func (v *View) remove(entity *Entity) {
	v.lock.RLock()
	for i, qr := range v.entities {
		if qr.Entity.ID == entity.ID {
			maxbound := len(v.entities) - 1
			v.entities[maxbound], v.entities[i] = v.entities[i], v.entities[maxbound]
			v.entities = v.entities[:maxbound]
			break
		}
	}
	v.lock.RUnlock()
}

type Manager struct {
	lock            *sync.RWMutex
	entityIdInc     uint32
	componentNumInc uint32 // limited to 64

	entities     []*Entity
	entitiesByID map[EntityID]*Entity
	components   []*Component
	views        map[string]*View
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

func (manager *Manager) CreateView(name string, tag Tag) *View {
	view := &View{
		tag:      tag,
		entities: make([]*QueryResult, 0),
		lock:     &sync.RWMutex{},
	}

	entities := manager.Query(tag)
	manager.lock.Lock()
	for _, entityresult := range entities {
		view.entities[entityresult.Entity.ID] = entityresult
	}
	manager.views[name] = view
	manager.lock.Unlock()

	return view
}

func (manager *Manager) View(name string) []*QueryResult {
	manager.lock.RLock()
	view, ok := manager.views[name]
	if !ok {
		manager.lock.RUnlock()
		return nil
	}

	return view.entities
}

type Entity struct {
	ID      EntityID
	tag     Tag
	manager *Manager
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
		views:           make(map[string]*View),
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
		ID:      EntityID(id),
		manager: manager,
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

	component.datalock.RLock()

	tagbefore := entity.tag
	entity.tag |= component.tag

	for _, view := range entity.manager.views {

		if !tagbefore.includes(view.tag) && entity.tag.includes(view.tag) {
			view.add(entity)
		}
	}

	component.datalock.RUnlock()
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
	tagbefore := entity.tag
	entity.tag ^= component.tag

	for _, view := range entity.manager.views {
		if tagbefore.includes(view.tag) && !entity.tag.includes(view.tag) {
			view.remove(entity)
		}
	}

	component.datalock.Unlock()
	return entity
}

func (entity Entity) HasComponent(component *Component) bool {
	return entity.tag&component.tag != 0x0000
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
	Components map[*Component]interface{}
}

func (manager *Manager) fetchComponentsForEntity(entity *Entity, tag Tag) map[*Component]interface{} {

	if entity.tag&tag != tag {
		return nil
	}

	componentMap := make(map[*Component]interface{})

	for _, component := range manager.components {
		if component.tag&tag == component.tag {
			data, ok := entity.GetComponentData(component)
			if !ok {
				return nil // if one of the required components is not set, return nothing !
			}

			componentMap[component] = data
		}

		// fmt.Printf("-------------\n")
		// fmt.Printf("%16b : %s\n", int64(tag), "tag")
		// fmt.Printf("%16b : %s\n", int64(component.tag), "component.tag")
		// fmt.Printf("%16b : %s\n", int64(entity.tag), "entity.tag")
		// fmt.Printf("//////////////////\n")
	}

	return componentMap
}

func (manager *Manager) Query(tag Tag) []*QueryResult {

	matches := make([]*QueryResult, 0)

	manager.lock.RLock()
	for _, entity := range manager.entities {
		if entity.tag&tag == tag {

			componentMap := make(map[*Component]interface{})

			for _, component := range manager.components {
				if component.tag&tag == component.tag {
					data, _ := entity.GetComponentData(component)
					componentMap[component] = data
				}
			}

			matches = append(matches, &QueryResult{
				Entity:     entity,
				Components: componentMap,
			})

		}
	}
	manager.lock.RUnlock()

	return matches
}
