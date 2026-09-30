package cli

import (
	"os"
	"strings"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/greenhouse"
	"github.com/thedavidweng/jobs-cli/v2/internal/workday"
)

func (a *App) applyProvider(name domain.ApplicationProvider) (domain.ApplyProvider, *joberrors.Error) {
	if name == domain.ProviderWorkday && a.browserEndpoint != "" {
		return workday.NewBrowserProvider(a.browserEndpoint, a.browserTab, a.browserState), nil
	}
	if name == domain.ProviderGreenhouse && a.greenhouseKeyFile != "" {
		if a.greenhouseBoard == "" {
			return nil, invalid("--greenhouse-board is required with --greenhouse-key-file")
		}
		key, err := os.ReadFile(a.greenhouseKeyFile)
		if err != nil {
			return nil, invalid("cannot read --greenhouse-key-file")
		}
		if strings.TrimSpace(string(key)) == "" {
			return nil, invalid("Greenhouse key file is empty")
		}
		return greenhouse.NewAuthorizedProvider(a.httpClient(), a.greenhouseBoard, strings.TrimSpace(string(key))), nil
	}
	return a.registry().Provider(name)
}
