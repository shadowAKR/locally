export namespace config {
	
	export class Settings {
	    ollamaUrl: string;
	    model: string;
	    autoApprove: boolean;
	    think: boolean;
	    numCtx: number;
	    temperature: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ollamaUrl = source["ollamaUrl"];
	        this.model = source["model"];
	        this.autoApprove = source["autoApprove"];
	        this.think = source["think"];
	        this.numCtx = source["numCtx"];
	        this.temperature = source["temperature"];
	    }
	}

}

export namespace main {
	
	export class ChatView {
	    id: string;
	    title: string;
	    source: string;
	    messages: ollama.Message[];
	
	    static createFrom(source: any = {}) {
	        return new ChatView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.source = source["source"];
	        this.messages = this.convertValues(source["messages"], ollama.Message);
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
	export class ModelEntry {
	    name: string;
	    size: string;
	    parameterSize: string;
	    supportsTools: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.parameterSize = source["parameterSize"];
	        this.supportsTools = source["supportsTools"];
	    }
	}
	export class Status {
	    connected: boolean;
	    ollamaVersion: string;
	    error?: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.ollamaVersion = source["ollamaVersion"];
	        this.error = source["error"];
	        this.model = source["model"];
	    }
	}

}

export namespace ollama {
	
	export class ToolCallFunction {
	    index?: number;
	    name: string;
	    arguments: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ToolCallFunction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.arguments = source["arguments"];
	    }
	}
	export class ToolCall {
	    function: ToolCallFunction;
	
	    static createFrom(source: any = {}) {
	        return new ToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.function = this.convertValues(source["function"], ToolCallFunction);
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
	export class Message {
	    role: string;
	    content: string;
	    thinking?: string;
	    tool_calls?: ToolCall[];
	    tool_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.thinking = source["thinking"];
	        this.tool_calls = this.convertValues(source["tool_calls"], ToolCall);
	        this.tool_name = source["tool_name"];
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

export namespace schedule {
	
	export class Task {
	    id: string;
	    name: string;
	    prompt: string;
	    kind: string;
	    time?: string;
	    // Go type: time
	    at?: any;
	    enabled: boolean;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastRun?: any;
	    lastChat?: string;
	    // Go type: time
	    nextRun?: any;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.prompt = source["prompt"];
	        this.kind = source["kind"];
	        this.time = source["time"];
	        this.at = this.convertValues(source["at"], null);
	        this.enabled = source["enabled"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastRun = this.convertValues(source["lastRun"], null);
	        this.lastChat = source["lastChat"];
	        this.nextRun = this.convertValues(source["nextRun"], null);
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

export namespace store {
	
	export class Summary {
	    id: string;
	    title: string;
	    source: string;
	    // Go type: time
	    updatedAt: any;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.source = source["source"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.count = source["count"];
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

