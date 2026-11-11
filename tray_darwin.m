//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

#define ANYPROXY_TRAY_TOGGLE 1
#define ANYPROXY_TRAY_OPEN   2
#define ANYPROXY_TRAY_QUIT   3

static NSImage *anyproxyCreateMenuBarIcon(BOOL active) {
	NSImage *img = [NSImage imageWithSize:NSMakeSize(18.0, 18.0)
	                              flipped:YES
	                       drawingHandler:^BOOL(NSRect dstRect) {
		(void)dstRect;
		CGContextRef cg = [[NSGraphicsContext currentContext] CGContext];
		[NSGraphicsContext saveGraphicsState];
		CGContextSetAlpha(cg, active ? 1.0 : 0.42);
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

@interface AnyProxyTrayController : NSObject
@property (nonatomic, strong) NSStatusItem *statusItem;
@property (nonatomic, strong) NSMenu *menu;
@property (nonatomic, strong) NSMenuItem *statusMenuItem;
@property (nonatomic, strong) NSMenuItem *toggleMenuItem;
@property (nonatomic, strong) NSImage *activeIcon;
@property (nonatomic, strong) NSImage *inactiveIcon;
@end

@implementation AnyProxyTrayController

- (instancetype)initWithActive:(BOOL)active
                   statusLabel:(NSString *)statusLabel
                   toggleLabel:(NSString *)toggleLabel
                       tooltip:(NSString *)tooltip {
	self = [super init];
	if (!self) {
		return nil;
	}

	_activeIcon = anyproxyCreateMenuBarIcon(YES);
	_inactiveIcon = anyproxyCreateMenuBarIcon(NO);

	_statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
	_menu = [[NSMenu alloc] initWithTitle:@"AnyProxy"];
	[_menu setAutoenablesItems:NO];

	_statusMenuItem = [[NSMenuItem alloc] initWithTitle:statusLabel action:nil keyEquivalent:@""];
	[_statusMenuItem setEnabled:NO];

	_toggleMenuItem = [[NSMenuItem alloc] initWithTitle:toggleLabel action:@selector(onToggle:) keyEquivalent:@""];
	[_toggleMenuItem setTarget:self];

	NSMenuItem *openItem = [[NSMenuItem alloc] initWithTitle:@"Open AnyProxy" action:@selector(onOpen:) keyEquivalent:@""];
	[openItem setTarget:self];

	NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit AnyProxy" action:@selector(onQuit:) keyEquivalent:@""];
	[quitItem setTarget:self];

	[_menu addItem:_statusMenuItem];
	[_menu addItem:_toggleMenuItem];
	[_menu addItem:[NSMenuItem separatorItem]];
	[_menu addItem:openItem];
	[_menu addItem:[NSMenuItem separatorItem]];
	[_menu addItem:quitItem];

	_statusItem.menu = _menu;
	[self applyActive:active statusLabel:statusLabel toggleLabel:toggleLabel tooltip:tooltip];
	return self;
}

- (void)applyActive:(BOOL)active
        statusLabel:(NSString *)statusLabel
        toggleLabel:(NSString *)toggleLabel
            tooltip:(NSString *)tooltip {
	if (_statusItem.button != nil) {
		_statusItem.button.image = active ? _activeIcon : _inactiveIcon;
		_statusItem.button.imagePosition = NSImageOnly;
		_statusItem.button.toolTip = tooltip;
	}
	[_statusMenuItem setTitle:statusLabel];
	[_toggleMenuItem setTitle:toggleLabel];
}

- (void)onToggle:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_TOGGLE);
}

- (void)onOpen:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_OPEN);
}

- (void)onQuit:(id)sender {
	(void)sender;
	anyproxyTrayMenuAction(ANYPROXY_TRAY_QUIT);
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

static NSString *anyproxyNSString(const char *s) {
	return s != NULL ? [NSString stringWithUTF8String:s] : @"";
}

void anyproxyTrayInit(int active, const char *statusLabel, const char *toggleLabel, const char *tooltip) {
	BOOL isActive = active != 0;
	NSString *status = anyproxyNSString(statusLabel);
	NSString *toggle = anyproxyNSString(toggleLabel);
	NSString *tip = anyproxyNSString(tooltip);

	anyproxyOnMainThread(^{
		if (gAnyProxyTray != nil) {
			[gAnyProxyTray applyActive:isActive statusLabel:status toggleLabel:toggle tooltip:tip];
			return;
		}
		gAnyProxyTray = [[AnyProxyTrayController alloc] initWithActive:isActive
		                                                   statusLabel:status
		                                                   toggleLabel:toggle
		                                                       tooltip:tip];
	});
}

void anyproxyTrayUpdate(int active, const char *statusLabel, const char *toggleLabel, const char *tooltip) {
	BOOL isActive = active != 0;
	NSString *status = anyproxyNSString(statusLabel);
	NSString *toggle = anyproxyNSString(toggleLabel);
	NSString *tip = anyproxyNSString(tooltip);

	anyproxyOnMainThread(^{
		if (gAnyProxyTray == nil) {
			return;
		}
		[gAnyProxyTray applyActive:isActive statusLabel:status toggleLabel:toggle tooltip:tip];
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

