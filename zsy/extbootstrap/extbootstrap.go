package extbootstrap

// Installed plugins. Adding a plugin requires adding a blank import here;
// core new-api files do not reference plugin packages by name.
import (
	_ "github.com/QuantumNous/new-api/zsy/appauth"
	_ "github.com/QuantumNous/new-api/zsy/avatar"
	_ "github.com/QuantumNous/new-api/zsy/runninghub"
	_ "github.com/QuantumNous/new-api/zsy/tone"
	_ "github.com/QuantumNous/new-api/zsy/voice"
	_ "github.com/QuantumNous/new-api/zsy/world"
)
