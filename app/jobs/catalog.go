package jobs

import (
	"github.com/AlexGalhardo/vukem/queue"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
)

// Catalog lists the catalog jobs (image thumbnails). Their handlers need
// the catalog service, which only exists once the app is built, so the
// catalog routes register them on app.Queue() instead of All.
func Catalog(svc *catalog.Service) []queue.Registration {
	return []queue.Registration{svc.ThumbnailRegistration()}
}
