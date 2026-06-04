// Feature versioning and lineage tracking for Phase 19B.
// Every feature has an immutable version history and a lineage graph
// that records which parent features and datasets it was derived from.
package featurestore

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Feature Version ──────────────────────────────────────────────────────────

// FeatureVersion captures an immutable snapshot of a feature definition at a
// specific version number. Once created, a version is never mutated.
type FeatureVersion struct {
	FeatureID   string
	Version     int
	Definition  FeatureDefinition
	ChangeNote  string
	CreatedBy   string
	CreatedAt   time.Time
}

// ─── Feature Lineage ──────────────────────────────────────────────────────────

// LineageNode represents a single node in the feature derivation graph.
type LineageNode struct {
	FeatureID  string
	Version    int
	ParentIDs  []string // features this feature was derived from
	DatasetIDs []string // datasets used to compute this feature
	Tags       []string
	CreatedAt  time.Time
}

// LineageGraph is the full directed acyclic graph of feature derivations.
type LineageGraph struct {
	mu    sync.RWMutex
	nodes map[string]*LineageNode // featureID → node
	edges map[string][]string     // featureID → []childFeatureIDs
}

// NewLineageGraph creates an empty feature lineage graph.
func NewLineageGraph() *LineageGraph {
	return &LineageGraph{
		nodes: make(map[string]*LineageNode),
		edges: make(map[string][]string),
	}
}

// Register adds a feature node to the lineage graph.
func (g *LineageGraph) Register(node LineageNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[node.FeatureID] = &node
	for _, parentID := range node.ParentIDs {
		g.edges[parentID] = append(g.edges[parentID], node.FeatureID)
	}
}

// Ancestors returns all ancestor feature IDs (transitive) for a given feature.
func (g *LineageGraph) Ancestors(featureID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	var traverse func(id string)
	traverse = func(id string) {
		node, ok := g.nodes[id]
		if !ok {
			return
		}
		for _, parentID := range node.ParentIDs {
			if !visited[parentID] {
				visited[parentID] = true
				traverse(parentID)
			}
		}
	}
	traverse(featureID)
	var out []string
	for id := range visited {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Descendants returns all downstream features derived from a given feature.
func (g *LineageGraph) Descendants(featureID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	var traverse func(id string)
	traverse = func(id string) {
		for _, childID := range g.edges[id] {
			if !visited[childID] {
				visited[childID] = true
				traverse(childID)
			}
		}
	}
	traverse(featureID)
	var out []string
	for id := range visited {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// GetNode returns the lineage node for a feature.
func (g *LineageGraph) GetNode(featureID string) (LineageNode, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[featureID]
	if !ok {
		return LineageNode{}, false
	}
	return *n, true
}

// ─── Version History ──────────────────────────────────────────────────────────

// VersionRegistry stores the immutable version history for all features.
type VersionRegistry struct {
	mu       sync.RWMutex
	versions map[string][]FeatureVersion // featureID → []versions (ascending)
}

// NewVersionRegistry creates an empty version registry.
func NewVersionRegistry() *VersionRegistry {
	return &VersionRegistry{versions: make(map[string][]FeatureVersion)}
}

// Commit records a new version of a feature definition.
// Version numbers are automatically assigned and always increment.
func (vr *VersionRegistry) Commit(def FeatureDefinition, changeNote, createdBy string) (FeatureVersion, error) {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	history := vr.versions[def.ID]
	nextVersion := 1
	if len(history) > 0 {
		nextVersion = history[len(history)-1].Version + 1
	}
	def.Version = nextVersion
	def.UpdatedAt = time.Now().UTC()

	fv := FeatureVersion{
		FeatureID:  def.ID,
		Version:    nextVersion,
		Definition: def,
		ChangeNote: changeNote,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now().UTC(),
	}
	vr.versions[def.ID] = append(vr.versions[def.ID], fv)
	return fv, nil
}

// GetVersion returns a specific version of a feature.
func (vr *VersionRegistry) GetVersion(featureID string, version int) (FeatureVersion, error) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	for _, fv := range vr.versions[featureID] {
		if fv.Version == version {
			return fv, nil
		}
	}
	return FeatureVersion{}, fmt.Errorf("featurestore: version %d not found for feature %s", version, featureID)
}

// GetLatestVersion returns the most recent version of a feature.
func (vr *VersionRegistry) GetLatestVersion(featureID string) (FeatureVersion, error) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	history := vr.versions[featureID]
	if len(history) == 0 {
		return FeatureVersion{}, fmt.Errorf("featurestore: no versions for feature %s", featureID)
	}
	return history[len(history)-1], nil
}

// ListVersions returns the full version history for a feature.
func (vr *VersionRegistry) ListVersions(featureID string) []FeatureVersion {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	out := make([]FeatureVersion, len(vr.versions[featureID]))
	copy(out, vr.versions[featureID])
	return out
}

// TotalVersions returns the total number of versions across all features.
func (vr *VersionRegistry) TotalVersions() int {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	total := 0
	for _, versions := range vr.versions {
		total += len(versions)
	}
	return total
}

// ─── Feature Registry ─────────────────────────────────────────────────────────

// Registry is a high-level interface combining the FeatureStore, VersionRegistry,
// and LineageGraph into a single cohesive feature management system.
type Registry struct {
	Store    *FeatureStore
	Versions *VersionRegistry
	Lineage  *LineageGraph
}

// NewRegistry creates a fully wired feature registry.
func NewRegistry() *Registry {
	return &Registry{
		Store:    NewFeatureStore(),
		Versions: NewVersionRegistry(),
		Lineage:  NewLineageGraph(),
	}
}

// Define registers a new feature and commits its initial version.
func (r *Registry) Define(def FeatureDefinition, parentIDs, datasetIDs []string,
	changeNote, createdBy string) (FeatureVersion, error) {

	fv, err := r.Versions.Commit(def, changeNote, createdBy)
	if err != nil {
		return FeatureVersion{}, err
	}
	// Sync version back to definition before registering in store.
	def.Version = fv.Version
	if err := r.Store.Register(def); err != nil {
		return FeatureVersion{}, err
	}
	r.Lineage.Register(LineageNode{
		FeatureID:  def.ID,
		Version:    fv.Version,
		ParentIDs:  parentIDs,
		DatasetIDs: datasetIDs,
		Tags:       def.Tags,
		CreatedAt:  fv.CreatedAt,
	})
	return fv, nil
}

// Update bumps a feature to a new version with updated parameters.
func (r *Registry) Update(featureID string, params map[string]any, changeNote, updatedBy string) (FeatureVersion, error) {
	existing, err := r.Store.GetDefinition(featureID)
	if err != nil {
		return FeatureVersion{}, err
	}
	existing.Parameters = params
	existing.UpdatedAt = time.Now().UTC()
	return r.Versions.Commit(existing, changeNote, updatedBy)
}
