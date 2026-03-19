// Package api provides HTTP handlers and the store-event watcher that keeps
// the widget registry and ASCII frame cache in sync with store changes.
package api

import (
	"context"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	asciiplugin "github.com/infocus7/omni/pkg/plugins/ascii"
	"github.com/infocus7/omni/pkg/store"
	"github.com/infocus7/omni/pkg/widgets"
)

// Watcher listens on a store.Watch channel and keeps the widget registry and
// ASCII frame cache consistent with store events.
// CRUD handlers also update eagerly for synchronous responses; the Watcher
// handles out-of-band changes from registry syncs and other out-of-band writes.
type Watcher struct {
	store    store.Store
	registry *widgets.Registry
	cache    *sync.Map // map["name/size"][]byte — gzip-compressed frames blob
	logger   zerolog.Logger
}

// NewWatcher creates a Watcher.
func NewWatcher(st store.Store, reg *widgets.Registry, cache *sync.Map, logger zerolog.Logger) *Watcher {
	return &Watcher{store: st, registry: reg, cache: cache, logger: logger}
}

// Start launches the watch goroutine. It exits when ctx is cancelled or the
// store's event channel is closed.
func (w *Watcher) Start(ctx context.Context) {
	go func() {
		ch, err := w.store.Watch(ctx)
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to start store watch")
			return
		}
		for ev := range ch {
			switch ev.Kind {
			case store.EventPut:
				// Re-register all variants so Definition().Sizes stays complete.
				if all, err := w.store.Get(ctx, ev.Name); err == nil && len(all) > 0 {
					w.registry.Register(asciiplugin.NewWidgetFromVariants(all))
				} else {
					w.registry.Register(asciiplugin.NewWidgetFromVariant(ev.Variant))
				}
				w.cache.Store(ev.Variant.Name+"/"+ev.Variant.Size, ev.Variant.FramesGzip)
				w.logger.Info().Str("animation", ev.Name).Str("size", ev.Variant.Size).Msg("store event: registered animation")

			case store.EventDelete:
				w.registry.Unregister("ascii-" + ev.Name)
				prefix := ev.Name + "/"
				w.cache.Range(func(k, _ any) bool {
					if strings.HasPrefix(k.(string), prefix) {
						w.cache.Delete(k)
					}
					return true
				})
				w.logger.Info().Str("animation", ev.Name).Msg("store event: removed animation")
			}
		}
	}()
}
