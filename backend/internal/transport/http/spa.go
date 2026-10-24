package http

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// spaHandler serves the built frontend (index.html + hashed assets) and falls
// back to index.html for client-side routes such as /admin and /watch.
func spaHandler(root string) fiber.Handler {
	root = filepath.Clean(root)
	index := filepath.Join(root, "index.html")
	return func(c *fiber.Ctx) error {
		p := c.Path()
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/hls/") || strings.HasPrefix(p, "/ws/") {
			return fiber.ErrNotFound
		}

		rel := strings.TrimPrefix(path.Clean("/"+p), "/")
		if rel == "" || filepath.Ext(rel) == "" {
			// A client-side route: the SPA router takes over.
			c.Set("Cache-Control", "no-cache")
			// The second argument keeps Accept-Encoding, so the compress
			// middleware can gzip the bundle.
			return c.SendFile(index, true)
		}

		full := filepath.Join(root, filepath.FromSlash(rel))
		if !strings.HasPrefix(full, root+string(os.PathSeparator)) && full != filepath.Clean(root) {
			return fiber.ErrNotFound
		}
		if _, err := os.Stat(full); err != nil {
			return fiber.ErrNotFound
		}
		if strings.HasPrefix(p, "/assets/") {
			// Vite asset names carry a content hash.
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Set("Cache-Control", "no-cache")
		}
		return c.SendFile(full, true)
	}
}
