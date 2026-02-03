export namespace main {
	
	export class StepResult {
	    svg: string;
	    canStep: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.svg = source["svg"];
	        this.canStep = source["canStep"];
	    }
	}

}

