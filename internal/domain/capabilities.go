package domain

// providerCapabilities is the canonical Capability table per Application Provider.
var providerCapabilities = map[ApplicationProvider]Capabilities{
	ProviderGreenhouse:      {Inspect: true, Prepare: true, NativeSubmit: true},
	ProviderLinkedIn:        {Inspect: true, Prepare: true, AuthRequired: true},
	ProviderIndeed:          {BrowserRequired: true},
	ProviderLever:           {BrowserRequired: true},
	ProviderAshby:           {BrowserRequired: true},
	ProviderWorkday:         {BrowserRequired: true},
	ProviderSmartRecruiters: {BrowserRequired: true},
	ProviderICIMS:           {BrowserRequired: true},
	ProviderExternal:        {BrowserRequired: true},
	ProviderUnknown:         {BrowserRequired: true},
}

// CapabilitiesFor returns the canonical Capability flags for an Application
// Provider; providers without an entry fall back to browser-required only.
func CapabilitiesFor(provider ApplicationProvider) Capabilities {
	if capabilities, ok := providerCapabilities[provider]; ok {
		return capabilities
	}
	return Capabilities{BrowserRequired: true}
}
