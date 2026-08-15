export namespace service {
	
	export class Status {
	    paired: boolean;
	    connected: boolean;
	    last_error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paired = source["paired"];
	        this.connected = source["connected"];
	        this.last_error = source["last_error"];
	    }
	}

}

