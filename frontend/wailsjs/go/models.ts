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
	export class JobEvent {
	    id: string;
	    type: string;
	    label: string;
	    status: string;
	    error?: string;
	    attempts?: number;
	    will_retry?: boolean;
	    // Go type: time
	    updated_at: any;

	    static createFrom(source: any = {}) {
	        return new JobEvent(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.attempts = source["attempts"];
	        this.will_retry = source["will_retry"];
	        this.updated_at = source["updated_at"];
	    }
	}

}

