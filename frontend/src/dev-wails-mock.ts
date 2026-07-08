type AnyFn = (...args: any[]) => any;

declare global {
  interface Window {
    go?: Record<string, Record<string, Record<string, AnyFn>>>;
    runtime?: Record<string, AnyFn>;
  }
}

const noop = () => undefined;
const resolved = <T>(value: T) => () => Promise.resolve(value);

const blMocks: Record<string, AnyFn> = {
  CoinVideo: resolved(false),
  FetchImage: resolved(""),
  Follow: resolved(false),
  GetAppVersion: resolved("dev"),
  GetBLFavFolderList: resolved([]),
  GetBLFavFolderListDetail: resolved([]),
  GetBLFeedList: resolved(undefined),
  GetBLHistoryList: resolved({ list: [], cursor: {} }),
  GetBLPopularList: resolved(undefined),
  GetBLRCMDList: resolved(undefined),
  GetBLUserInfo: resolved(undefined),
  GetBiliTicket: resolved(""),
  GetCList: resolved({ pages: [] }),
  GetDanmakuList: resolved({ items: [] }),
  GetImageProxyPort: resolved(0),
  GetLoginQRCode: resolved(""),
  GetLoginQRCodeStatus: resolved(undefined),
  GetLoginStatus: resolved(false),
  GetPlaylist: resolved("[]"),
  GetPlaylistPlayMode: resolved("sequence"),
  GetReplyList: resolved({ items: [], has_more: false, next: 1 }),
  GetSESSDATA: resolved(""),
  GetSeriesList: resolved([]),
  GetSeriesVideos: resolved([]),
  GetUpVideoList: resolved(undefined),
  GetUrlByCid: resolved({ url: "" }),
  HasCoin: resolved(0),
  HasLiked: resolved(false),
  HmacSha256: resolved(""),
  IsFollowing: resolved(false),
  LikeVideo: resolved(false),
  ProxyImage: resolved(""),
  ReportPlayProgress: resolved(undefined),
  SearchVideo: resolved([]),
  SetLoginStatus: resolved(undefined),
  SetPlaylist: resolved(undefined),
  SetPlaylistPlayMode: resolved(undefined),
  SetSESSDATA: resolved(undefined),
  Unfollow: resolved(false),
};

const menuMocks: Record<string, AnyFn> = {
  CheckForUpdates: resolved(undefined),
  CloseApp: resolved(undefined),
  GetPlatform: resolved("darwin"),
  SetAppContext: resolved(undefined),
  ShowAbout: resolved(undefined),
  ShowKeyboardShortcuts: resolved(undefined),
  ShowVersion: resolved(undefined),
};

export function installDevWailsMock() {
  if (!import.meta.env.DEV || window.go || window.runtime) return;

  window.go = {
    service: {
      BL: blMocks,
    },
    main: {
      Menu: menuMocks,
    },
  };

  window.runtime = {
    BrowserOpenURL: noop,
    ClipboardGetText: resolved(""),
    ClipboardSetText: noop,
    CloseApp: noop,
    Environment: resolved({}),
    EventsEmit: noop,
    EventsOff: noop,
    EventsOffAll: noop,
    EventsOn: noop,
    EventsOnMultiple: noop,
    EventsOnce: noop,
    Hide: noop,
    LogDebug: noop,
    LogError: noop,
    LogFatal: noop,
    LogInfo: noop,
    LogPrint: noop,
    LogTrace: noop,
    LogWarning: noop,
    Quit: noop,
    ScreenGetAll: resolved([]),
    Show: noop,
    WindowCenter: noop,
    WindowFullscreen: noop,
    WindowGetPosition: resolved({ x: 0, y: 0 }),
    WindowGetSize: resolved({ w: 800, h: 600 }),
    WindowHide: noop,
    WindowIsFullscreen: resolved(false),
    WindowIsMaximised: resolved(false),
    WindowIsMinimised: resolved(false),
    WindowIsNormal: resolved(true),
    WindowMaximise: noop,
    WindowMinimise: noop,
    WindowReload: noop,
    WindowReloadApp: noop,
    WindowSetAlwaysOnTop: noop,
    WindowSetBackgroundColour: noop,
    WindowSetDarkTheme: noop,
    WindowSetLightTheme: noop,
    WindowSetMaxSize: noop,
    WindowSetMinSize: noop,
    WindowSetPosition: noop,
    WindowSetSize: noop,
    WindowSetSystemDefaultTheme: noop,
    WindowSetTitle: noop,
    WindowShow: noop,
    WindowToggleMaximise: noop,
    WindowUnfullscreen: noop,
    WindowUnmaximise: noop,
    WindowUnminimise: noop,
  };
}
