// SPDX-License-Identifier: AGPL-3.0-or-later

#import <Foundation/Foundation.h>
#import <NetworkExtension/NetworkExtension.h>
#import <Security/Security.h>
#import <SystemExtensions/SystemExtensions.h>
#include <string.h>
#include "ne_bridge_darwin.h"

static void write_cstr(char *buf, int len, NSString *str) {
    if (!buf || len <= 0) return;
    if (!str) {
        buf[0] = '\0';
        return;
    }
    const char *utf8 = [str UTF8String];
    if (!utf8) {
        buf[0] = '\0';
        return;
    }
    strlcpy(buf, utf8, (size_t)len);
}

static BOOL has_sysext_entitlement(void) {
    SecTaskRef task = SecTaskCreateFromSelf(NULL);
    if (!task) return NO;
    CFTypeRef val = SecTaskCopyValueForEntitlement(task, CFSTR("com.apple.developer.system-extension.install"), NULL);
    CFRelease(task);
    if (!val) return NO;
    BOOL ok = (CFGetTypeID(val) == CFBooleanGetTypeID()) && CFBooleanGetValue((CFBooleanRef)val);
    CFRelease(val);
    return ok;
}

int anyproxy_ne_available(const char *extBundleID) {
    @autoreleasepool {
        if (!extBundleID || !has_sysext_entitlement()) return 0;
        NSString *extID = [NSString stringWithUTF8String:extBundleID];
        NSString *bundlePath = [[NSBundle mainBundle] bundlePath];
        if (!bundlePath) return 0;
        NSString *extPath = [[bundlePath stringByAppendingPathComponent:@"Contents/Library/SystemExtensions"]
            stringByAppendingPathComponent:[NSString stringWithFormat:@"%@.systemextension", extID]];
        BOOL isDir = NO;
        return ([[NSFileManager defaultManager] fileExistsAtPath:extPath isDirectory:&isDir] && isDir) ? 1 : 0;
    }
}

@interface AnyProxySysExtDelegate : NSObject <OSSystemExtensionRequestDelegate>
@property (nonatomic, strong) dispatch_semaphore_t sema;
@property (nonatomic, assign) BOOL succeeded;
@property (nonatomic, assign) BOOL needsApproval;
@property (nonatomic, copy) NSString *errorMessage;
@end

@implementation AnyProxySysExtDelegate
- (void)request:(OSSystemExtensionRequest *)request didFinishWithResult:(OSSystemExtensionRequestResult)result {
    self.succeeded = YES;
    dispatch_semaphore_signal(self.sema);
}
- (void)request:(OSSystemExtensionRequest *)request didFailWithError:(NSError *)error {
    self.succeeded = NO;
    self.errorMessage = error.localizedDescription ?: @"System extension activation failed";
    dispatch_semaphore_signal(self.sema);
}
- (void)requestNeedsUserApproval:(OSSystemExtensionRequest *)request {
    self.needsApproval = YES;
    self.errorMessage = @"System Extension requires one-time approval: open System Settings -> General -> Login Items & Extensions -> scroll to Network Extensions -> click (i) -> enable AnyProxy Network Extension.";
    dispatch_semaphore_signal(self.sema);
}
- (OSSystemExtensionReplacementAction)request:(OSSystemExtensionRequest *)request
                  actionForReplacingExtension:(OSSystemExtensionProperties *)existing
                                withExtension:(OSSystemExtensionProperties *)ext {
    return OSSystemExtensionReplacementActionReplace;
}
@end

static NETransparentProxyManager *load_proxy_manager(NSError **outErr) {
    __block NETransparentProxyManager *found = nil;
    __block NSError *loadErr = nil;
    dispatch_semaphore_t sema = dispatch_semaphore_create(0);
    [NETransparentProxyManager loadAllFromPreferencesWithCompletionHandler:^(NSArray<NETransparentProxyManager *> *managers, NSError *error) {
        loadErr = error;
        found = managers.firstObject;
        dispatch_semaphore_signal(sema);
    }];
    dispatch_semaphore_wait(sema, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
    if (outErr) *outErr = loadErr;
    return found;
}

int anyproxy_ne_start(const char *extBundleID, const char *localizedName, char *errBuf, int errLen) {
    @autoreleasepool {
        NSString *extID = [NSString stringWithUTF8String:extBundleID ?: "dev.anyproxy.app.extension"];
        NSString *desc = [NSString stringWithUTF8String:localizedName ?: "AnyProxy"];

        AnyProxySysExtDelegate *del = [[AnyProxySysExtDelegate alloc] init];
        del.sema = dispatch_semaphore_create(0);
        dispatch_queue_t q = dispatch_queue_create("dev.anyproxy.sysext", DISPATCH_QUEUE_SERIAL);
        OSSystemExtensionRequest *req = [OSSystemExtensionRequest activationRequestForExtension:extID queue:q];
        req.delegate = del;
        [[OSSystemExtensionManager sharedManager] submitRequest:req];

        if (dispatch_semaphore_wait(del.sema, dispatch_time(DISPATCH_TIME_NOW, 30 * NSEC_PER_SEC)) != 0) {
            write_cstr(errBuf, errLen, @"Timed out waiting for System Extension activation");
            return -1;
        }
        if (!del.succeeded) {
            write_cstr(errBuf, errLen, del.errorMessage ?: @"System Extension activation failed");
            return -1;
        }

        NSError *err = nil;
        NETransparentProxyManager *manager = load_proxy_manager(&err);
        if (err) {
            write_cstr(errBuf, errLen, err.localizedDescription);
            return -1;
        }
        if (!manager) {
            manager = [[NETransparentProxyManager alloc] init];
        }

        NETunnelProviderProtocol *proto = (NETunnelProviderProtocol *)manager.protocolConfiguration;
        if (!proto || ![proto isKindOfClass:[NETunnelProviderProtocol class]]) {
            proto = [[NETunnelProviderProtocol alloc] init];
        }
        proto.providerBundleIdentifier = extID;
        proto.serverAddress = @"127.0.0.1";
        manager.protocolConfiguration = proto;
        manager.localizedDescription = desc;
        manager.enabled = YES;

        __block NSError *saveErr = nil;
        dispatch_semaphore_t saveSema = dispatch_semaphore_create(0);
        [manager saveToPreferencesWithCompletionHandler:^(NSError *error) {
            saveErr = error;
            dispatch_semaphore_signal(saveSema);
        }];
        dispatch_semaphore_wait(saveSema, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
        if (saveErr) {
            write_cstr(errBuf, errLen, saveErr.localizedDescription);
            return -1;
        }

        __block NSError *reloadErr = nil;
        dispatch_semaphore_t reloadSema = dispatch_semaphore_create(0);
        [manager loadFromPreferencesWithCompletionHandler:^(NSError *error) {
            reloadErr = error;
            dispatch_semaphore_signal(reloadSema);
        }];
        dispatch_semaphore_wait(reloadSema, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
        if (reloadErr) {
            write_cstr(errBuf, errLen, reloadErr.localizedDescription);
            return -1;
        }

        // Configure NEDNSProxyManager so DNS queries map IP <-> domain cleanly.
        NEDNSProxyManager *dns = [NEDNSProxyManager sharedManager];
        dispatch_semaphore_t dnsSema = dispatch_semaphore_create(0);
        [dns loadFromPreferencesWithCompletionHandler:^(NSError *error) {
            if (!error) {
                NEDNSProxyProviderProtocol *dnsProto = [[NEDNSProxyProviderProtocol alloc] init];
                dnsProto.providerBundleIdentifier = extID;
                dns.providerProtocol = dnsProto;
                dns.localizedDescription = [NSString stringWithFormat:@"%@ DNS", desc];
                dns.enabled = YES;
                [dns saveToPreferencesWithCompletionHandler:^(NSError *saveDnsErr) {
                    dispatch_semaphore_signal(dnsSema);
                }];
            } else {
                dispatch_semaphore_signal(dnsSema);
            }
        }];
        dispatch_semaphore_wait(dnsSema, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));

        NSError *startErr = nil;
        if (![manager.connection startVPNTunnelAndReturnError:&startErr]) {
            write_cstr(errBuf, errLen, startErr.localizedDescription ?: @"Failed to start transparent proxy tunnel");
            return -1;
        }
        return 0;
    }
}

int anyproxy_ne_stop(char *errBuf, int errLen) {
    @autoreleasepool {
        NSError *err = nil;
        NETransparentProxyManager *manager = load_proxy_manager(&err);
        if (err) {
            write_cstr(errBuf, errLen, err.localizedDescription);
            return -1;
        }
        if (manager) {
            [manager.connection stopVPNTunnel];
        }
        NEDNSProxyManager *dns = [NEDNSProxyManager sharedManager];
        dispatch_semaphore_t dnsSema = dispatch_semaphore_create(0);
        [dns loadFromPreferencesWithCompletionHandler:^(NSError *loadErr) {
            if (!loadErr && dns.enabled) {
                dns.enabled = NO;
                [dns saveToPreferencesWithCompletionHandler:^(NSError *saveErr) {
                    dispatch_semaphore_signal(dnsSema);
                }];
            } else {
                dispatch_semaphore_signal(dnsSema);
            }
        }];
        dispatch_semaphore_wait(dnsSema, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
        return 0;
    }
}

int anyproxy_ne_status(int *outState, char *msgBuf, int msgLen) {
    @autoreleasepool {
        NSError *err = nil;
        NETransparentProxyManager *manager = load_proxy_manager(&err);
        if (err) {
            if (outState) *outState = 0;
            write_cstr(msgBuf, msgLen, err.localizedDescription);
            return -1;
        }
        if (!manager) {
            if (outState) *outState = (int)NEVPNStatusDisconnected;
            return 0;
        }
        if (outState) *outState = (int)manager.connection.status;
        return 0;
    }
}

int anyproxy_ne_send_message(const char *jsonStr, char *outBuf, int outLen, char *errBuf, int errLen) {
    @autoreleasepool {
        if (!jsonStr) {
            write_cstr(errBuf, errLen, @"Empty message");
            return -1;
        }
        NSError *err = nil;
        NETransparentProxyManager *manager = load_proxy_manager(&err);
        if (err || !manager) {
            write_cstr(errBuf, errLen, err ? err.localizedDescription : @"Transparent proxy manager not configured");
            return -1;
        }
        if (![manager.connection isKindOfClass:[NETunnelProviderSession class]]) {
            write_cstr(errBuf, errLen, @"Connection is not a NETunnelProviderSession");
            return -1;
        }
        NETunnelProviderSession *session = (NETunnelProviderSession *)manager.connection;
        NSData *data = [NSData dataWithBytes:jsonStr length:strlen(jsonStr)];

        __block NSData *respData = nil;
        __block BOOL completed = NO;
        dispatch_semaphore_t sema = dispatch_semaphore_create(0);
        NSError *sendErr = nil;
        BOOL ok = [session sendProviderMessage:data returnError:&sendErr responseHandler:^(NSData *responseData) {
            respData = responseData;
            completed = YES;
            dispatch_semaphore_signal(sema);
        }];
        if (!ok) {
            write_cstr(errBuf, errLen, sendErr.localizedDescription ?: @"sendProviderMessage failed");
            return -1;
        }
        if (dispatch_semaphore_wait(sema, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC)) != 0 || !completed) {
            write_cstr(errBuf, errLen, @"Timed out waiting for extension response");
            return -1;
        }
        if (outBuf && outLen > 0) {
            if (respData && respData.length > 0) {
                NSUInteger copyLen = MIN(respData.length, (NSUInteger)(outLen - 1));
                memcpy(outBuf, respData.bytes, copyLen);
                outBuf[copyLen] = '\0';
            } else {
                outBuf[0] = '\0';
            }
        }
        return 0;
    }
}
