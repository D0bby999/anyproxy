// SPDX-License-Identifier: AGPL-3.0-or-later

#ifndef ANYPROXY_NE_BRIDGE_DARWIN_H
#define ANYPROXY_NE_BRIDGE_DARWIN_H

// Returns 1 if the .systemextension bundle is embedded and the host app holds
// the com.apple.developer.system-extension.install entitlement.
int anyproxy_ne_available(const char *extBundleID);

// Activates the system extension, configures NETransparentProxyManager +
// NEDNSProxyManager, and starts the transparent proxy tunnel.
// Returns 0 on success, -1 on error (with message in errBuf).
int anyproxy_ne_start(const char *extBundleID, const char *localizedName, char *errBuf, int errLen);

// Stops the transparent proxy tunnel and disables the DNS proxy manager.
int anyproxy_ne_stop(char *errBuf, int errLen);

// Queries the current NEVPNStatus (0..5) into outState.
int anyproxy_ne_status(int *outState, char *msgBuf, int msgLen);

// Sends a JSON provider message to the running NETransparentProxyProvider.
// outLen receives the response bytes (if any). Returns 0 on success, -1 on error.
int anyproxy_ne_send_message(const char *jsonStr, char *outBuf, int outLen, char *errBuf, int errLen);

#endif
