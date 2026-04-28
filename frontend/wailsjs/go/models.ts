export namespace config {
	
	export class Config {
	    api_key?: string;
	    default_mode?: string;
	    default_voice?: string;
	    default_bot_name?: string;
	    trigger_words?: string;
	    context?: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_key = source["api_key"];
	        this.default_mode = source["default_mode"];
	        this.default_voice = source["default_voice"];
	        this.default_bot_name = source["default_bot_name"];
	        this.trigger_words = source["trigger_words"];
	        this.context = source["context"];
	    }
	}

}

