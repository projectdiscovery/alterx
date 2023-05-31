package runner

import (
	"github.com/projectdiscovery/gologger"
	updateutils "github.com/projectdiscovery/utils/update"
)

var banner = `
   ___   ____          _  __
  / _ | / / /____ ____| |/_/
 / __ |/ / __/ -_) __/>  <  
/_/ |_/_/\__/\__/_/ /_/|_|  				 
`

var version = "v0.0.2"

// showBanner is used to show the banner to the user
func showBanner() {
	gologger.Print().Msgf("%s\n", banner)
	gologger.Print().Msgf("\t\tprojectdiscovery.io\n\n")
}

// GetUpdateCallback returns a callback function that updates alterx
func GetUpdateCallback() func() {
	return func() {
		showBanner()
		updateutils.GetUpdateToolCallback("alterx", version)()
	}
}
