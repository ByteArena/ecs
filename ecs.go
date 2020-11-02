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

type Tag struct {
	flags   uint64 // limited to 64 components
	inverse bool
}

func (tag Tag) matches(smallertag Tag) bool {
	res := tag.flags&smallertag.flags == smallertag.flags

	if smallertag.inverse {
		return !res
	}

	return res
}

func (tag *Tag) binaryORInPlace(othertag Tag) *Tag {
	tag.flags |= othertag.flags
	return tag
}

func (tag *Tag) binaryNOTInPlace(othertag Tag) *Tag {
	tag.flags ^= othertag.flags
	return tag
}

func (tag Tag) clone() Tag {
	return tag
}

func (tag Tag) Inverse(values ...bool) Tag {

	clone := tag.clone()
	inverse := true
	if len(values) > 0 {
		inverse = values[0]
	}

	clone.inverse = inverse
	return clone
}

type View struct {
	tag      Tag
	entities QueryResultCollection
	lock     *sync.RWMutex
}

type QueryResultCollection []*QueryResult

func (coll QueryResultCollection) Entities() []*Entity {
	res := make([]*Entity, len(coll))
	for i, qr := range coll {
		res[i] = qr.Entity
	}

	return res
}

func (v View) Get() QueryResultCollection {
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
	views        []*View
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

func (manager *Manager) CreateView(tagelements ...interface{}) *View {

	tag := BuildTag(tagelements...)

	view := &View{
		tag:  tag,
		lock: &sync.RWMutex{},
	}

	entities := manager.Query(tag)
	view.entities = make(QueryResultCollection, len(entities))
	manager.lock.Lock()
	for i, entityresult := range entities {
		view.entities[i] = entityresult
	}
	manager.views = append(manager.views, view)
	manager.lock.Unlock()

	return view
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
		views:           make([]*View, 0),
	}
}

func BuildTag(elements ...interface{}) Tag {

	tag := Tag{}

	for _, element := range elements {
		switch typedelement := element.(type) {
		case *Component:
			{
				tag.binaryORInPlace(typedelement.tag)
			}
		case Tag:
			{
				tag.binaryORInPlace(typedelement)
			}
		default:
			{
				panic("Invalid type passed to BuildTag; accepts only <*Component> and <Tag> types.")
			}
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

	tag := Tag{
		flags:   (1 << id), // set bit on position corresponding to component number
		inverse: false,
	}

	component := &Component{
		id:       id,
		tag:      tag,
		data:     make(map[EntityID]interface{}),
		datalock: &sync.RWMutex{},
	}

	manager.lock.Lock()
	manager.components = append(manager.components, component)
	manager.lock.Unlock()

	return component
}

func (manager Manager) GetEntityByID(id EntityID, tagelements ...interface{}) *QueryResult {

	manager.lock.RLock()
	res, ok := manager.entitiesByID[id]

	if !ok {
		manager.lock.RUnlock()
		return nil
	}

	tag := BuildTag(tagelements...)

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

func (entity Entity) Matches(tag Tag) bool {
	return entity.tag.matches(tag)
}

func (entity *Entity) AddComponent(component *Component, componentdata interface{}) *Entity {
	component.datalock.Lock()
	component.data[entity.ID] = componentdata
	component.datalock.Unlock()

	component.datalock.RLock()

	tagbefore := entity.tag
	entity.tag.binaryORInPlace(component.tag)

	for _, view := range entity.manager.views {

		if !tagbefore.matches(view.tag) && entity.tag.matches(view.tag) {
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
	entity.tag.binaryNOTInPlace(component.tag)

	for _, view := range entity.manager.views {
		if tagbefore.matches(view.tag) && !entity.tag.matches(view.tag) {
			view.remove(entity)
		}
	}

	component.datalock.Unlock()
	return entity
}

func (entity Entity) HasComponent(component *Component) bool {
	return entity.tag.matches(component.tag)
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

func (manager *Manager) DisposeEntity(entity interface{}) {

	var typedentity *Entity

	switch typeditem := entity.(type) {
	case *QueryResult:
		{
			typedentity = typeditem.Entity
		}
	case QueryResult:
		{
			typedentity = typeditem.Entity
		}
	case *Entity:
		{
			typedentity = typeditem
		}
	default:
		{
			panic("Invalid type passed to DisposeEntity; accepts only <*QueryResult>, <QueryResult> and <*Entity> types.")
		}
	}

	if typedentity == nil {
		return
	}

	manager.lock.Lock()
	for _, component := range manager.components {
		if typedentity.HasComponent(component) {
			typedentity.RemoveComponent(component)
		}
	}
	delete(manager.entitiesByID, typedentity.ID)
	manager.lock.Unlock()
}

type QueryResult struct {
	Entity     *Entity
	Components map[*Component]interface{}
}

func (manager *Manager) fetchComponentsForEntity(entity *Entity, tag Tag) map[*Component]interface{} {

	if !entity.tag.matches(tag) {
		return nil
	}

	componentMap := make(map[*Component]interface{})

	for _, component := range manager.components {
		if tag.matches(component.tag) {
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

func (manager *Manager) Query(tag Tag) QueryResultCollection {

	matches := make(QueryResultCollection, 0)

	manager.lock.RLock()
	for _, entity := range manager.entities {
		if entity.tag.matches(tag) {

			componentMap := make(map[*Component]interface{})

			for _, component := range manager.components {
				if tag.matches(component.tag) {
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
