export namespace service {
	
	export class AppVersion {
	    version: string;
	    build: number;
	
	    static createFrom(source: any = {}) {
	        return new AppVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.build = source["build"];
	    }
	}
	export class DanmakuItem {
	    content: string;
	    time: number;
	    type: number;
	    fontSize: number;
	    color: number;
	    sendTime: number;
	    poolType: number;
	    senderHash: string;
	    dmid: number;
	
	    static createFrom(source: any = {}) {
	        return new DanmakuItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.time = source["time"];
	        this.type = source["type"];
	        this.fontSize = source["fontSize"];
	        this.color = source["color"];
	        this.sendTime = source["sendTime"];
	        this.poolType = source["poolType"];
	        this.senderHash = source["senderHash"];
	        this.dmid = source["dmid"];
	    }
	}
	export class DanmakuList {
	    items: DanmakuItem[];
	
	    static createFrom(source: any = {}) {
	        return new DanmakuList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], DanmakuItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FeedList {
	    items: any[];
	    has_more: boolean;
	    offset: string;
	
	    static createFrom(source: any = {}) {
	        return new FeedList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.has_more = source["has_more"];
	        this.offset = source["offset"];
	    }
	}
	export class FollowStatus {
	    is_following: boolean;
	    follower: number;
	
	    static createFrom(source: any = {}) {
	        return new FollowStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_following = source["is_following"];
	        this.follower = source["follower"];
	    }
	}
	export class HistoryList {
	    list: any[];
	    // Go type: struct { Max int "json:\"max\""; ViewAt int "json:\"view_at\""; Business string "json:\"business\"" }
	    cursor: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = source["list"];
	        this.cursor = this.convertValues(source["cursor"], Object);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Page {
	    cid: number;
	    page: number;
	    from: string;
	    part: string;
	    duration: number;
	    vid: string;
	    weblink: string;
	    // Go type: struct { Width int "json:\"width\""; Height int "json:\"height\""; Rotate int "json:\"rotate\"" }
	    dimension: any;
	    first_frame: string;
	
	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cid = source["cid"];
	        this.page = source["page"];
	        this.from = source["from"];
	        this.part = source["part"];
	        this.duration = source["duration"];
	        this.vid = source["vid"];
	        this.weblink = source["weblink"];
	        this.dimension = this.convertValues(source["dimension"], Object);
	        this.first_frame = source["first_frame"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlayURLInfo {
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayURLInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	    }
	}
	export class PopularList {
	    items: any[];
	    has_more: boolean;
	    no_more: boolean;
	    page: number;
	
	    static createFrom(source: any = {}) {
	        return new PopularList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.has_more = source["has_more"];
	        this.no_more = source["no_more"];
	        this.page = source["page"];
	    }
	}
	export class RCMDList {
	    items: any[];
	    has_more: boolean;
	    page: number;
	
	    static createFrom(source: any = {}) {
	        return new RCMDList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.has_more = source["has_more"];
	        this.page = source["page"];
	    }
	}
	export class ReplyContent {
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ReplyContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	    }
	}
	export class ReplyItem {
	    rpid: number;
	    oid: number;
	    type: number;
	    mid: number;
	    content: ReplyContent;
	    ctime: number;
	    like: number;
	    action: number;
	    member: any;
	    replies: ReplyItem[];
	    root: number;
	    parent: number;
	
	    static createFrom(source: any = {}) {
	        return new ReplyItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rpid = source["rpid"];
	        this.oid = source["oid"];
	        this.type = source["type"];
	        this.mid = source["mid"];
	        this.content = this.convertValues(source["content"], ReplyContent);
	        this.ctime = source["ctime"];
	        this.like = source["like"];
	        this.action = source["action"];
	        this.member = source["member"];
	        this.replies = this.convertValues(source["replies"], ReplyItem);
	        this.root = source["root"];
	        this.parent = source["parent"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReplyList {
	    items: ReplyItem[];
	    has_more: boolean;
	    next: number;
	    total_count: number;
	
	    static createFrom(source: any = {}) {
	        return new ReplyList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], ReplyItem);
	        this.has_more = source["has_more"];
	        this.next = source["next"];
	        this.total_count = source["total_count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SearchResult {
	    picture_url: string;
	    url: string;
	    title: string;
	    views: string;
	    danmuCount: number;
	    author: string;
	    date: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.picture_url = source["picture_url"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.views = source["views"];
	        this.danmuCount = source["danmuCount"];
	        this.author = source["author"];
	        this.date = source["date"];
	    }
	}
	export class SeriesVideosResponse_Archive {
	    aid: number;
	    bvid: string;
	    title: string;
	    pubdate: number;
	    duration: number;
	    pic: string;
	    // Go type: struct { View int "json:\"view\"" }
	    stat: any;
	
	    static createFrom(source: any = {}) {
	        return new SeriesVideosResponse_Archive(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.aid = source["aid"];
	        this.bvid = source["bvid"];
	        this.title = source["title"];
	        this.pubdate = source["pubdate"];
	        this.duration = source["duration"];
	        this.pic = source["pic"];
	        this.stat = this.convertValues(source["stat"], Object);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UserInfo {
	    uname: string;
	    face: string;
	    mid: number;
	
	    static createFrom(source: any = {}) {
	        return new UserInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uname = source["uname"];
	        this.face = source["face"];
	        this.mid = source["mid"];
	    }
	}
	export class VideoInfo {
	    bvid: string;
	    aid: number;
	    title: string;
	    desc: string;
	    videos: number;
	    pic: string;
	    owner_mid: number;
	    owner_name: string;
	    owner_face: string;
	    pages: Page[];
	    cid: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bvid = source["bvid"];
	        this.aid = source["aid"];
	        this.title = source["title"];
	        this.desc = source["desc"];
	        this.videos = source["videos"];
	        this.pic = source["pic"];
	        this.owner_mid = source["owner_mid"];
	        this.owner_name = source["owner_name"];
	        this.owner_face = source["owner_face"];
	        this.pages = this.convertValues(source["pages"], Page);
	        this.cid = source["cid"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

