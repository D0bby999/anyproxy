//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

#import <Cocoa/Cocoa.h>
#include <string.h>
#include "_cgo_export.h"

#define ANYPROXY_TRAY_TOGGLE  1
#define ANYPROXY_TRAY_OPEN    2
#define ANYPROXY_TRAY_COMPACT 3
#define ANYPROXY_TRAY_QUIT    4

static NSImage *anyproxyCreateMenuBarIcon(BOOL active) {
	NSImage *img = [NSImage imageWithSize:NSMakeSize(18.0, 18.0)
	                              flipped:YES
	                       drawingHandler:^BOOL(NSRect dstRect) {
		(void)dstRect;
		CGContextRef cg = [[NSGraphicsContext currentContext] CGContext];
		[NSGraphicsContext saveGraphicsState];
		CGContextSetAlpha(cg, active ? 1.0 : 0.45);
		CGContextBeginTransparencyLayer(cg, NULL);

		NSAffineTransform *tx = [NSAffineTransform transform];
		[tx translateXBy:9.0 yBy:9.2];
		[tx scaleBy:(16.5 / 241.0)];
		[tx translateXBy:-190.5 yBy:-211.5];
		[tx concat];

		NSColor *ink = [NSColor blackColor];
		[ink setFill];
		[ink setStroke];

		// Curled tail
		NSBezierPath *tail = [NSBezierPath bezierPath];
		[tail moveToPoint:NSMakePoint(142.0, 222.0)];
		[tail curveToPoint:NSMakePoint(82.0, 255.0)
		     controlPoint1:NSMakePoint(108.0, 218.0)
		     controlPoint2:NSMakePoint(82.0, 231.0)];
		[tail curveToPoint:NSMakePoint(117.0, 283.0)
		     controlPoint1:NSMakePoint(82.0, 276.0)
		     controlPoint2:NSMakePoint(99.0, 287.0)];
		[tail curveToPoint:NSMakePoint(134.0, 258.0)
		     controlPoint1:NSMakePoint(133.0, 280.0)
		     controlPoint2:NSMakePoint(139.0, 268.0)];
		[tail curveToPoint:NSMakePoint(116.0, 257.0)
		     controlPoint1:NSMakePoint(130.0, 250.0)
		     controlPoint2:NSMakePoint(119.0, 250.0)];
		[tail setLineWidth:24.0];
		[tail setLineCapStyle:NSLineCapStyleRound];
		[tail stroke];

		// Legs
		NSBezierPath *legs = [NSBezierPath bezierPath];
		[legs moveToPoint:NSMakePoint(161.0, 222.0)];
		[legs lineToPoint:NSMakePoint(153.0, 266.0)];
		[legs lineToPoint:NSMakePoint(139.0, 285.0)];
		[legs moveToPoint:NSMakePoint(232.0, 223.0)];
		[legs lineToPoint:NSMakePoint(240.0, 267.0)];
		[legs lineToPoint:NSMakePoint(253.0, 285.0)];
		[legs setLineWidth:19.0];
		[legs setLineCapStyle:NSLineCapStyleRound];
		[legs setLineJoinStyle:NSLineJoinStyleRound];
		[legs stroke];

		// Body, crest, snout
		NSBezierPath *body = [NSBezierPath bezierPath];
		[body moveToPoint:NSMakePoint(124.0, 218.0)];
		[body curveToPoint:NSMakePoint(187.0, 153.0)
		     controlPoint1:NSMakePoint(124.0, 181.0)
		     controlPoint2:NSMakePoint(148.0, 153.0)];
		[body curveToPoint:NSMakePoint(235.0, 172.0)
		     controlPoint1:NSMakePoint(206.0, 153.0)
		     controlPoint2:NSMakePoint(222.0, 160.0)];
		[body curveToPoint:NSMakePoint(256.0, 136.0)
		     controlPoint1:NSMakePoint(243.0, 164.0)
		     controlPoint2:NSMakePoint(249.0, 149.0)];
		[body curveToPoint:NSMakePoint(275.0, 135.0)
		     controlPoint1:NSMakePoint(260.0, 128.0)
		     controlPoint2:NSMakePoint(270.0, 127.0)];
		[body lineToPoint:NSMakePoint(287.0, 154.0)];
		[body curveToPoint:NSMakePoint(311.0, 180.0)
		     controlPoint1:NSMakePoint(301.0, 157.0)
		     controlPoint2:NSMakePoint(311.0, 167.0)];
		[body curveToPoint:NSMakePoint(288.0, 201.0)
		     controlPoint1:NSMakePoint(311.0, 193.0)
		     controlPoint2:NSMakePoint(302.0, 201.0)];
		[body lineToPoint:NSMakePoint(275.0, 201.0)];
		[body curveToPoint:NSMakePoint(203.0, 247.0)
		     controlPoint1:NSMakePoint(265.0, 227.0)
		     controlPoint2:NSMakePoint(240.0, 244.0)];
		[body curveToPoint:NSMakePoint(124.0, 218.0)
		     controlPoint1:NSMakePoint(159.0, 250.0)
		     controlPoint2:NSMakePoint(124.0, 241.0)];
		[body closePath];
		[body fill];

		// Eye cutout
		[[NSGraphicsContext currentContext] setCompositingOperation:NSCompositingOperationClear];
		NSBezierPath *eye = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(264.0, 157.0, 22.0, 22.0)];
		[eye fill];

		CGContextEndTransparencyLayer(cg);
		[NSGraphicsContext restoreGraphicsState];
		return YES;
	}];
	[img setTemplate:YES];
	return img;
}

static NSString *anyproxyDictString(NSDictionary *dict, NSString *key, NSString *fallback) {
	id val = dict[key];
	return [val isKindOfClass:[NSString class]] ? (NSString *)val : fallback;
}

static BOOL anyproxyDictBool(NSDictionary *dict, NSString *key) {
	id val = dict[key];
	return [val isKindOfClass:[NSNumber class]] ? [(NSNumber *)val boolValue] : NO;
}

@interface AnyProxyTrayController : NSObject <NSMenuDelegate>
@property (nonatomic, strong) NSStatusItem *statusItem;
@property (nonatomic, strong) NSMenu *menu;
@property (nonatomic, strong) NSImage *activeIcon;
@property (nonatomic, strong) NSImage *inactiveIcon;
@property (nonatomic, copy) NSString *listenAddr;
@property (nonatomic, assign) BOOL menuOpen;
@property (nonatomic, strong) NSDictionary *pendingSnapshot;
@end

@implementation AnyProxyTrayController

- (instancetype)initWithJSONData:(NSData *)data {
	self = [super init];
	if (!self) {
		return nil;
	}

	_activeIcon = anyproxyCreateMenuBarIcon(YES);
	_inactiveIcon = anyproxyCreateMenuBarIcon(NO);
	_listenAddr = @"127.0.0.1:10800";

	_statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
	_menu = [[NSMenu alloc] initWithTitle:@"AnyProxy"];
	[_menu setAutoenablesItems:NO];
	[_menu setDelegate:self];

	_statusItem.menu = _menu;
	[self applyJSONData:data];
	return self;
}

- (void)menuWillOpen:(NSMenu *)menu {
	(void)menu;
	_menuOpen = YES;
}

- (void)menuDidClose:(NSMenu *)menu {
	(void)menu;
	_menuOpen = NO;
	if (_pendingSnapshot != nil) {
		NSDictionary *snap = _pendingSnapshot;
		_pendingSnapshot = nil;
		[self rebuildMenuWithSnapshot:snap];
	}
}

- (void)applyJSONData:(NSData *)data {
	if (data == nil) {
		return;
	}
	id parsed = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
	if (![parsed isKindOfClass:[NSDictionary class]]) {
		return;
	}
	NSDictionary *snap = (NSDictionary *)parsed;

	BOOL active = anyproxyDictBool(snap, @"active");
	NSString *tooltip = anyproxyDictString(snap, @"tooltip", @"AnyProxy");
	if (_statusItem.button != nil) {
		_statusItem.button.image = active ? _activeIcon : _inactiveIcon;
		_statusItem.button.imagePosition = NSImageOnly;
		_statusItem.button.toolTip = tooltip;
	}

	_listenAddr = anyproxyDictString(snap, @"listenAddr", @"127.0.0.1:10800");
	if (_menuOpen) {
		_pendingSnapshot = snap;
		return;
	}
	[self rebuildMenuWithSnapshot:snap];
}

- (void)rebuildMenuWithSnapshot:(NSDictionary *)snap {
	[_menu removeAllItems];

	NSString *statusLabel = anyproxyDictString(snap, @"statusLabel", @"Status: Stopped");
	NSString *toggleLabel = anyproxyDictString(snap, @"toggleLabel", @"Enable AnyProxy");
	NSString *activeProxyText = anyproxyDictString(snap, @"activeProxyText", @"Direct");
	NSString *defaultProxyID = anyproxyDictString(snap, @"defaultProxyId", @"");
	NSString *activeProfile = anyproxyDictString(snap, @"activeProfile", @"Default");
	NSString *listenAddr = anyproxyDictString(snap, @"listenAddr", @"127.0.0.1:10800");
	BOOL compact = anyproxyDictBool(snap, @"compact");

	NSMenuItem *statusItem = [[NSMenuItem alloc] initWithTitle:statusLabel action:nil keyEquivalent:@""];
	[statusItem setEnabled:NO];
	[_menu addItem:statusItem];

	NSMenuItem *toggleItem = [[NSMenuItem alloc] initWithTitle:toggleLabel action:@selector(onToggle:) keyEquivalent:@""];
	[toggleItem setTarget:self];
	[_menu addItem:toggleItem];

	[_menu addItem:[NSMenuItem separatorItem]];

	// Active Proxy submenu
	NSMenuItem *proxyParent = [[NSMenuItem alloc] initWithTitle:[NSString stringWithFormat:@"Active Proxy: %@", activeProxyText]
	                                                     action:nil
	                                              keyEquivalent:@""];
	NSMenu *proxyMenu = [[NSMenu alloc] initWithTitle:@"Active Proxy"];
	[proxyMenu setAutoenablesItems:NO];
	NSMenuItem *directItem = [[NSMenuItem alloc] initWithTitle:@"Direct (No Proxy)"
	                                                    action:@selector(onSelectProxy:)
	                                             keyEquivalent:@""];
	[directItem setTarget:self];
	[directItem setRepresentedObject:@""];
	[directItem setState:(defaultProxyID.length == 0 ? NSControlStateValueOn : NSControlStateValueOff)];
	[proxyMenu addItem:directItem];

	id proxiesVal = snap[@"proxies"];
	if ([proxiesVal isKindOfClass:[NSArray class]] && [(NSArray *)proxiesVal count] > 0) {
		[proxyMenu addItem:[NSMenuItem separatorItem]];
		for (id entry in (NSArray *)proxiesVal) {
			if (![entry isKindOfClass:[NSDictionary class]]) {
				continue;
			}
			NSDictionary *px = (NSDictionary *)entry;
			NSString *pxID = anyproxyDictString(px, @"id", @"");
			NSString *pxLabel = anyproxyDictString(px, @"label", @"Proxy");
			if (pxID.length == 0) {
				continue;
			}
			NSMenuItem *pxItem = [[NSMenuItem alloc] initWithTitle:pxLabel
			                                                action:@selector(onSelectProxy:)
			                                         keyEquivalent:@""];
			[pxItem setTarget:self];
			[pxItem setRepresentedObject:pxID];
			[pxItem setState:([pxID isEqualToString:defaultProxyID] ? NSControlStateValueOn : NSControlStateValueOff)];
			[proxyMenu addItem:pxItem];
		}
	}
	[proxyParent setSubmenu:proxyMenu];
	[_menu addItem:proxyParent];

	// Profile submenu
	NSMenuItem *profileParent = [[NSMenuItem alloc] initWithTitle:[NSString stringWithFormat:@"Profile: %@", activeProfile]
	                                                       action:nil
	                                                keyEquivalent:@""];
	NSMenu *profileMenu = [[NSMenu alloc] initWithTitle:@"Profile"];
	[profileMenu setAutoenablesItems:NO];
	id profilesVal = snap[@"profiles"];
	if ([profilesVal isKindOfClass:[NSArray class]]) {
		for (id entry in (NSArray *)profilesVal) {
			if (![entry isKindOfClass:[NSString class]]) {
				continue;
			}
			NSString *name = (NSString *)entry;
			NSMenuItem *profItem = [[NSMenuItem alloc] initWithTitle:name
			                                                  action:@selector(onSelectProfile:)
			                                           keyEquivalent:@""];
			[profItem setTarget:self];
			[profItem setRepresentedObject:name];
			[profItem setState:([name isEqualToString:activeProfile] ? NSControlStateValueOn : NSControlStateValueOff)];
			[profileMenu addItem:profItem];
		}
	}
	[profileParent setSubmenu:profileMenu];
	[_menu addItem:profileParent];

	NSMenuItem *copyAddrItem = [[NSMenuItem alloc] initWithTitle:[NSString stringWithFormat:@"Copy Proxy Address (%@)", listenAddr]
	                                                      action:@selector(onCopyAddress:)
	                                               keyEquivalent:@""];
	[copyAddrItem setTarget:self];
	[_menu addItem:copyAddrItem];

	[_menu addItem:[NSMenuItem separatorItem]];

	NSMenuItem *openItem = [[NSMenuItem alloc] initWithTitle:@"Open AnyProxy" action:@selector(onOpen:) keyEquivalent:@""];
	[openItem setTarget:self];
	[_menu addItem:openItem];

	NSMenuItem *compactItem = [[NSMenuItem alloc] initWithTitle:@"Compact Mode" action:@selector(onCompact:) keyEquivalent:@""];
	[compactItem setTarget:self];
	[compactItem setState:(compact ? NSControlStateValueOn : NSControlStateValueOff)];
	[_menu addItem:compactItem];

	[_menu addItem:[NSMenuItem separatorItem]];

	NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit AnyProxy" action:@selector(onQuit:) keyEquivalent:@""];
	[quitItem setTarget:self];
	[_menu addItem:quitItem];
}

- (void)onToggle:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_TOGGLE);
}

- (void)onOpen:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_OPEN);
}

- (void)onCompact:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_COMPACT);
}

- (void)onQuit:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_QUIT);
}

- (void)onCopyAddress:(id)sender {
	(void)sender;
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	[pb clearContents];
	[pb setString:(_listenAddr.length > 0 ? _listenAddr : @"127.0.0.1:10800") forType:NSPasteboardTypeString];
}

- (void)onSelectProxy:(NSMenuItem *)sender {
	NSString *proxyID = [sender representedObject];
	if ([proxyID isKindOfClass:[NSString class]]) {
		anyproxyTraySelectProxy((char *)[proxyID UTF8String]);
	}
}

- (void)onSelectProfile:(NSMenuItem *)sender {
	NSString *profileName = [sender representedObject];
	if ([profileName isKindOfClass:[NSString class]]) {
		anyproxyTraySelectProfile((char *)[profileName UTF8String]);
	}
}

@end

static AnyProxyTrayController *gAnyProxyTray = nil;

static void anyproxyOnMainThread(dispatch_block_t block) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_async(dispatch_get_main_queue(), block);
	}
}

static NSData *anyproxyCopyUTF8Data(const char *json) {
	if (json == NULL) {
		return nil;
	}
	return [NSData dataWithBytes:json length:strlen(json)];
}

void anyproxyTrayInitJSON(const char *json) {
	NSData *data = anyproxyCopyUTF8Data(json);
	anyproxyOnMainThread(^{
		if (gAnyProxyTray != nil) {
			[gAnyProxyTray applyJSONData:data];
			return;
		}
		gAnyProxyTray = [[AnyProxyTrayController alloc] initWithJSONData:data];
	});
}

void anyproxyTrayUpdateJSON(const char *json) {
	NSData *data = anyproxyCopyUTF8Data(json);
	anyproxyOnMainThread(^{
		if (gAnyProxyTray == nil) {
			return;
		}
		[gAnyProxyTray applyJSONData:data];
	});
}

void anyproxyTrayActivateApp(void) {
	anyproxyOnMainThread(^{
		if (@available(macOS 14.0, *)) {
			[NSApp activate];
		} else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
		}
	});
}

void anyproxyShowAboutPanel(void) {
	anyproxyOnMainThread(^{
		[NSApp orderFrontStandardAboutPanel:nil];
		if (@available(macOS 14.0, *)) {
			[NSApp activate];
		} else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
		}
	});
}

void anyproxyTrayDestroy(void) {
	anyproxyOnMainThread(^{
		if (gAnyProxyTray != nil) {
			[[NSStatusBar systemStatusBar] removeStatusItem:gAnyProxyTray.statusItem];
			gAnyProxyTray = nil;
		}
	});
}

void anyproxyTrayWakeRunLoop(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSEvent *ev = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
		                                 location:NSZeroPoint
		                            modifierFlags:0
		                                timestamp:0
		                             windowNumber:0
		                                  context:nil
		                                  subtype:0
		                                    data1:0
		                                    data2:0];
		[NSApp postEvent:ev atStart:YES];
	});
}

