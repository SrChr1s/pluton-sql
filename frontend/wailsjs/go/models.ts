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
	    autoIncrement: boolean;
	
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
	        this.autoIncrement = source["autoIncrement"];
	    }
	}
	export class ForeignKeyInfo {
	    name: string;
	    columns: string[];
	    refSchema: string;
	    refTable: string;
	    refColumns: string[];
	    onUpdate: string;
	    onDelete: string;
	
	    static createFrom(source: any = {}) {
	        return new ForeignKeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.refSchema = source["refSchema"];
	        this.refTable = source["refTable"];
	        this.refColumns = source["refColumns"];
	        this.onUpdate = source["onUpdate"];
	        this.onDelete = source["onDelete"];
	    }
	}
	export class IndexInfo {
	    name: string;
	    columns: string[];
	    unique: boolean;
	    primary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IndexInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.unique = source["unique"];
	        this.primary = source["primary"];
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
	
	export class ApplyResult {
	    updated: number;
	    inserted: number;
	    deleted: number;
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	        this.inserted = source["inserted"];
	        this.deleted = source["deleted"];
	    }
	}
	export class Change {
	    kind: string;
	    key: Record<string, any>;
	    values: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Change(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.key = source["key"];
	        this.values = source["values"];
	    }
	}
	export class TableRef {
	    profile: string;
	    database: string;
	    schema: string;
	    table: string;
	
	    static createFrom(source: any = {}) {
	        return new TableRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	    }
	}
	export class PageRequest {
	    table: TableRef;
	    offset: number;
	    limit: number;
	    orderBy: string;
	    desc: boolean;
	    filter: string;
	
	    static createFrom(source: any = {}) {
	        return new PageRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = this.convertValues(source["table"], TableRef);
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	        this.orderBy = source["orderBy"];
	        this.desc = source["desc"];
	        this.filter = source["filter"];
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
	export class TablePage {
	    columns: driver.ColumnInfo[];
	    rows: any[][];
	    hasMore: boolean;
	    keyColumns: string[];
	    hasPk: boolean;
	    isView: boolean;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new TablePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = this.convertValues(source["columns"], driver.ColumnInfo);
	        this.rows = source["rows"];
	        this.hasMore = source["hasMore"];
	        this.keyColumns = source["keyColumns"];
	        this.hasPk = source["hasPk"];
	        this.isView = source["isView"];
	        this.durationMs = source["durationMs"];
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
	
	export class TableStructure {
	    columns: driver.ColumnInfo[];
	    indexes: driver.IndexInfo[];
	    foreignKeys: driver.ForeignKeyInfo[];
	    ddl: string;
	    ddlError: string;
	    isView: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TableStructure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = this.convertValues(source["columns"], driver.ColumnInfo);
	        this.indexes = this.convertValues(source["indexes"], driver.IndexInfo);
	        this.foreignKeys = this.convertValues(source["foreignKeys"], driver.ForeignKeyInfo);
	        this.ddl = source["ddl"];
	        this.ddlError = source["ddlError"];
	        this.isView = source["isView"];
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

