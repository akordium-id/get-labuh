package settings

import (
	"context"
	"io"

	"github.com/a-h/templ"
	"github.com/akordium-id/get-labuh/internal/models"
)

func serverStatusBadge(status models.ServerStatus) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		switch status {
		case models.ServerStatusOnline:
			_, err := w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800\">Online</span>"))
			return err
		case models.ServerStatusError:
			_, err := w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800\">Error</span>"))
			return err
		default:
			_, err := w.Write([]byte("<span class=\"inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800\">Offline</span>"))
			return err
		}
	})
}
