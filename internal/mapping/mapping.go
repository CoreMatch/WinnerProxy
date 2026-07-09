package mapping

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/winnerproxy/winnerproxy/config"
	"github.com/winnerproxy/winnerproxy/internal/cache"
	"github.com/winnerproxy/winnerproxy/internal/hrpauth"
	"github.com/winnerproxy/winnerproxy/internal/proxy"
)

type PlayerMapping struct {
	EntryID               string `json:"entry_id"`
	DeclaredYggdrasilTree string `json:"declared_yggdrasil_tree"`
	UpstreamName          string `json:"upstream_name"`
	UpstreamUUID          string `json:"upstream_uuid"`
	DownstreamName        string `json:"downstream_name"`
	DownstreamUUID        string `json:"downstream_uuid"`
	AlwaysPermit          bool   `json:"always_permit"`
}

type Mapping struct {
	cache      *cache.Cache
	cfg        *config.Config
	services   []proxy.UpstreamService
	serviceMap map[string]proxy.UpstreamService
}

func New(cache *cache.Cache, cfg *config.Config, services []proxy.UpstreamService) *Mapping {
	serviceMap := make(map[string]proxy.UpstreamService)
	for _, s := range services {
		serviceMap[s.ID()] = s
	}
	return &Mapping{cache: cache, cfg: cfg, services: services, serviceMap: serviceMap}
}

func (m *Mapping) generateEntryID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func (m *Mapping) generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0F) | 0x40
	b[8] = (b[8] & 0x3F) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (m *Mapping) getMappingKey(serviceID, upstreamUUID string) string {
	return fmt.Sprintf("mapping:%s:%s", serviceID, upstreamUUID)
}

func (m *Mapping) getDownstreamKey(downstreamUUID string) string {
	return fmt.Sprintf("downstream:%s", downstreamUUID)
}

func (m *Mapping) Transform(service proxy.UpstreamService, profile *hrpauth.PlayerProfile) (*hrpauth.PlayerProfile, error) {
	if m.cfg.Mapping.MojangToExternal && service.ID() == "mojang" {
		return m.transformMojangToExternal(service, profile)
	}

	return m.transformDefault(service, profile)
}

func (m *Mapping) transformDefault(service proxy.UpstreamService, profile *hrpauth.PlayerProfile) (*hrpauth.PlayerProfile, error) {
	key := m.getMappingKey(service.ID(), profile.ID)

	if data, err := m.cache.Get([]byte(key)); err == nil {
		var mapping PlayerMapping
		if err := json.Unmarshal(data, &mapping); err == nil {
			return &hrpauth.PlayerProfile{
				ID:         mapping.DownstreamUUID,
				Name:       mapping.DownstreamName,
				Properties: profile.Properties,
			}, nil
		}
	}

	downstreamName := profile.Name
	downstreamUUID := profile.ID

	if m.cfg.Mapping.AutoResolveName {
		for i := 0; i < 40; i++ {
			if !m.isNameExists(downstreamName) {
				break
			}
			if i == 0 {
				downstreamName = profile.Name + "_" + service.ID()
			} else {
				downstreamName = fmt.Sprintf("%s_%s_%d", profile.Name, service.ID(), i)
			}
		}
	}

	if m.cfg.Mapping.AutoResolveUUID {
		for i := 0; i < 40; i++ {
			if !m.isUUIDExists(downstreamUUID) {
				break
			}
			downstreamUUID = m.generateUUID()
		}
	}

	mapping := PlayerMapping{
		EntryID:               m.generateEntryID(),
		DeclaredYggdrasilTree: service.ID(),
		UpstreamName:          profile.Name,
		UpstreamUUID:          profile.ID,
		DownstreamName:        downstreamName,
		DownstreamUUID:        downstreamUUID,
		AlwaysPermit:          false,
	}

	data, _ := json.Marshal(mapping)
	_ = m.cache.Set([]byte(key), data, 0)

	downstreamKey := m.getDownstreamKey(downstreamUUID)
	_ = m.cache.Set([]byte(downstreamKey), data, 0)

	return &hrpauth.PlayerProfile{
		ID:         downstreamUUID,
		Name:       downstreamName,
		Properties: profile.Properties,
	}, nil
}

func (m *Mapping) transformMojangToExternal(service proxy.UpstreamService, profile *hrpauth.PlayerProfile) (*hrpauth.PlayerProfile, error) {
	externalService := m.serviceMap[m.cfg.Mapping.ExternalServiceID]
	if externalService == nil {
		return m.transformDefault(service, profile)
	}

	externalProfiles, err := externalService.BatchQuery([]string{profile.Name})
	if err != nil || len(externalProfiles) == 0 {
		return m.transformDefault(service, profile)
	}

	externalProfile := externalProfiles[0]

	key := m.getMappingKey(externalService.ID(), externalProfile.ID)

	if data, err := m.cache.Get([]byte(key)); err == nil {
		var mapping PlayerMapping
		if err := json.Unmarshal(data, &mapping); err == nil {
			return &hrpauth.PlayerProfile{
				ID:         mapping.DownstreamUUID,
				Name:       mapping.DownstreamName,
				Properties: profile.Properties,
			}, nil
		}
	}

	mapping := PlayerMapping{
		EntryID:               m.generateEntryID(),
		DeclaredYggdrasilTree: externalService.ID(),
		UpstreamName:          externalProfile.Name,
		UpstreamUUID:          externalProfile.ID,
		DownstreamName:        externalProfile.Name,
		DownstreamUUID:        externalProfile.ID,
		AlwaysPermit:          false,
	}

	data, _ := json.Marshal(mapping)
	_ = m.cache.Set([]byte(key), data, 0)

	downstreamKey := m.getDownstreamKey(externalProfile.ID)
	_ = m.cache.Set([]byte(downstreamKey), data, 0)

	return &hrpauth.PlayerProfile{
		ID:         externalProfile.ID,
		Name:       externalProfile.Name,
		Properties: profile.Properties,
	}, nil
}

func (m *Mapping) isNameExists(name string) bool {
	for _, s := range m.services {
		key := fmt.Sprintf("name:%s:%s", s.ID(), name)
		if _, err := m.cache.Get([]byte(key)); err == nil {
			return true
		}
	}
	return false
}

func (m *Mapping) isUUIDExists(uuid string) bool {
	key := m.getDownstreamKey(uuid)
	_, err := m.cache.Get([]byte(key))
	return err == nil
}

func (m *Mapping) QueryByDownstreamUUID(downstreamUUID string) (*PlayerMapping, error) {
	key := m.getDownstreamKey(downstreamUUID)
	data, err := m.cache.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	var mapping PlayerMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}
	return &mapping, nil
}
