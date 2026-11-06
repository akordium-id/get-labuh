package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

type PruneOptions struct {
	PruneContainers bool `json:"prune_containers"`
	PruneImages     bool `json:"prune_images"`
	PruneBuildCache bool `json:"prune_build_cache"`
	PruneNetworks   bool `json:"prune_networks"`
}

func DefaultPruneOptions() PruneOptions {
	return PruneOptions{
		PruneContainers: true,
		PruneImages:     true,
		PruneBuildCache: true,
		PruneNetworks:   true,
	}
}

type PruneReport struct {
	ContainersDeleted []string  `json:"containers_deleted"`
	ImagesDeleted     []string  `json:"images_deleted"`
	SpaceReclaimed    uint64    `json:"space_reclaimed_bytes"`
	SpaceFormatted    string    `json:"space_reclaimed_formatted"`
	Timestamp         time.Time `json:"timestamp"`
}

// FormatBytes mengubah angka byte menjadi representasi string B, KB, MB, GB
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	val := float64(bytes) / float64(div)
	switch exp {
	case 0:
		return fmt.Sprintf("%.2f KB", val)
	case 1:
		return fmt.Sprintf("%.2f MB", val)
	case 2:
		return fmt.Sprintf("%.2f GB", val)
	default:
		return fmt.Sprintf("%.2f TB", val)
	}
}

// PruneSystem menjalankan pembersihan resource docker (containers, dangling images, build cache, networks)
func (c *Client) PruneSystem(ctx context.Context, opts PruneOptions) (*PruneReport, error) {
	if c.cli == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	report := &PruneReport{
		Timestamp: time.Now(),
	}

	var totalReclaimed uint64

	// 1. Prune stopped containers
	if opts.PruneContainers {
		cReport, err := c.cli.ContainersPrune(ctx, filters.NewArgs())
		if err == nil {
			report.ContainersDeleted = append(report.ContainersDeleted, cReport.ContainersDeleted...)
			totalReclaimed += cReport.SpaceReclaimed
		}
	}

	// 2. Prune dangling images ONLY (safe for instant rollback cache)
	if opts.PruneImages {
		imgFilter := filters.NewArgs()
		imgFilter.Add("dangling", "true")
		iReport, err := c.cli.ImagesPrune(ctx, imgFilter)
		if err == nil {
			for _, img := range iReport.ImagesDeleted {
				if img.Deleted != "" {
					report.ImagesDeleted = append(report.ImagesDeleted, img.Deleted)
				} else if img.Untagged != "" {
					report.ImagesDeleted = append(report.ImagesDeleted, img.Untagged)
				}
			}
			totalReclaimed += iReport.SpaceReclaimed
		}
	}

	// 3. Prune Docker build cache
	if opts.PruneBuildCache {
		bReport, err := c.cli.BuildCachePrune(ctx, types.BuildCachePruneOptions{All: true})
		if err == nil && bReport != nil {
			totalReclaimed += bReport.SpaceReclaimed
		}
	}

	// 4. Prune unused networks
	if opts.PruneNetworks {
		_, _ = c.cli.NetworksPrune(ctx, filters.NewArgs())
	}

	report.SpaceReclaimed = totalReclaimed
	report.SpaceFormatted = FormatBytes(totalReclaimed)

	return report, nil
}

