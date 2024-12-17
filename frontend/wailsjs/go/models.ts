export namespace main {
	
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

