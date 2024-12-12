export namespace main {
	
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

}

