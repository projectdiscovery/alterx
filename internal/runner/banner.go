package runner

import (
	"github.com/projectdiscovery/gologger"
	updateutils "github.com/projectdiscovery/utils/update"
)

var banner = (`
        __   __                       
_____  |  |_/  |_  ____  ___ ___   ___
\__  \ |  |\   __\/ __ \_  __ \  \/  /
 / __ \|  |_|  | \  ___/|  | \/>    < 
(____  /____/__|  \___  >__|  /__/\_ \
     \/               \/            \/							 
`)

var version = "v0.0.1"

// showBanner is used to show the banner to the user
func showBanner() {
	gologger.Print().Msgf("%s\n", banner)
	gologger.Print().Msgf("\t\tprojectdiscovery.io\n\n")
}

// GetUpdateCallback returns a callback function that updates katana
func GetUpdateCallback() func() {
	return func() {
		showBanner()
		updateutils.GetUpdateToolCallback("alterx", version)()
	}
}
