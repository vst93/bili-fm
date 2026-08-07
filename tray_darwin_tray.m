//go:build darwin

// Objective-C 实现：macOS 状态栏托盘（NSStatusItem）。
// 已从 tray_darwin.go 的 cgo preamble 拆出，避免 cgo //export 导致的对象重复编译。
// AppKit 必须在主线程；Wails OnStartup 在非主线程 goroutine。
// 用 dispatch_async（非阻塞）切到主队列，避免事件循环未启动时 dispatch_sync 死锁。

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>

// Go 侧通过 //export 提供的回调
void trayShowWindow(void);
void trayQuit(void);

@interface TrayHelper : NSObject
- (void)showWindow:(id)sender;
- (void)quitApp:(id)sender;
@end
@implementation TrayHelper
- (void)showWindow:(id)sender { trayShowWindow(); }
- (void)quitApp:(id)sender { trayQuit(); }
@end

static NSStatusItem *g_statusItem = nil;
static TrayHelper *g_helper = nil;

static void doInitAppTrayWithPath(NSString *iconPath) {
    // 确保应用激活（状态栏项需要应用处于前台运行才会显示）
    [NSApp activateIgnoringOtherApps:YES];
    g_helper = [[TrayHelper alloc] init];
    NSStatusBar *bar = [NSStatusBar systemStatusBar];
    g_statusItem = [bar statusItemWithLength:NSVariableStatusItemLength];

    // 使用自定义图标替代系统默认图标
    if (iconPath && [[NSFileManager defaultManager] fileExistsAtPath:iconPath]) {
        NSImage *customIcon = [[NSImage alloc] initWithContentsOfFile:iconPath];
        if (customIcon) {
            [customIcon setSize:NSMakeSize(18, 18)];
            g_statusItem.button.image = customIcon;
        } else {
            NSImage *image = [NSImage imageNamed:@"NSApplicationIcon"];
            [image setSize:NSMakeSize(18, 18)];
            g_statusItem.button.image = image;
        }
    } else {
        NSImage *image = [NSImage imageNamed:@"NSApplicationIcon"];
        [image setSize:NSMakeSize(18, 18)];
        g_statusItem.button.image = image;
    }

    // 左键：显示主窗口
    g_statusItem.button.target = g_helper;
    g_statusItem.button.action = @selector(showWindow:);

    // 右键/control-click：弹出菜单（NSButton.menu，不影响左键 action）
    NSMenu *menu = [[NSMenu alloc] init];
    NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"显示窗口" action:@selector(showWindow:) keyEquivalent:@""];
    showItem.target = g_helper;
    [menu addItem:showItem];
    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"退出" action:@selector(quitApp:) keyEquivalent:@""];
    quitItem.target = g_helper;
    [menu addItem:quitItem];
    [g_statusItem.button setMenu:menu];

    NSLog(@"bili-FM tray: created on main=%d, item=%@",
          [[NSThread currentThread] isMainThread],
          g_statusItem ? @"yes" : @"nil");
}

void initAppTray(const char* iconPath) {
    if (g_statusItem) return;
    NSString *path = iconPath ? [NSString stringWithUTF8String:iconPath] : nil;
    void (^createBlock)(void) = ^{
        // 诊断：打印应用激活状态
        NSLog(@"bili-FM tray: isActive before create = %d", [NSApp isActive]);
        doInitAppTrayWithPath(path);
        // 再激活一次，确保前台
        [NSApp activateIgnoringOtherApps:YES];
        NSLog(@"bili-FM tray: isActive after create = %d", [NSApp isActive]);
    };
    if ([[NSThread currentThread] isMainThread]) {
        createBlock();
    } else {
        // 延迟到应用完全启动激活后创建（2s），避免 runloop/激活时序问题
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(2.0 * NSEC_PER_SEC)),
                       dispatch_get_main_queue(), createBlock);
    }
}

static void doDestroyAppTray(void) {
    if (g_statusItem) {
        [[NSStatusBar systemStatusBar] removeStatusItem:g_statusItem];
        g_statusItem = nil;
    }
}

void destroyAppTray(void) {
    if (!g_statusItem) return;
    if ([[NSThread currentThread] isMainThread]) {
        doDestroyAppTray();
    } else {
        dispatch_sync(dispatch_get_main_queue(), ^{
            doDestroyAppTray();
        });
    }
}