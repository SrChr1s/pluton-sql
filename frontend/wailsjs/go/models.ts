export namespace config {
	
	export class ConnectionProfile {
	    id: string;
	    name: string;
	    driver: string;
	    host: string;
	    port: number;
	    user: string;
	    database: string;
	    filePath: string;
	    sslMode: string;
	    tlsMinVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.database = source["database"];
	        this.filePath = source["filePath"];
	        this.sslMode = source["sslMode"];
	        this.tlsMinVersion = source["tlsMinVersion"];
	    }
	}

}

export namespace driver {
	
	export class ColumnInfo {
	    name: string;
	    type: string;
	    nullable: boolean;
	    primaryKey: boolean;
	    default: string;
	
	    static createFrom(source: any = {}) {
	        return new ColumnInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.nullable = source["nullable"];
	        this.primaryKey = source["primaryKey"];
	        this.default = source["default"];
	    }
	}
	export class IndexInfo {
	    name: string;
	    columns: string[];
	    unique: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IndexInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.unique = source["unique"];
	    }
	}
	export class TableInfo {
	    name: string;
	    schema: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new TableInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.schema = source["schema"];
	        this.type = source["type"];
	    }
	}

}

export namespace service {
	
	export class QueryResult {
	    columns: string[];
	    rows: any[][];
	    rowsAffected: number;
	    truncated: boolean;
	    durationMs: number;
	    isQuery: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.truncated = source["truncated"];
	        this.durationMs = source["durationMs"];
	        this.isQuery = source["isQuery"];
	    }
	}

}

