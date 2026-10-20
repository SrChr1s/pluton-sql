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
	    color: string;
	    workspace: string;
	
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
	        this.color = source["color"];
	        this.workspace = source["workspace"];
	    }
	}
	export class Workspace {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
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

export namespace history {
	
	export class Entry {
	    id: string;
	    profile: string;
	    database: string;
	    sql: string;
	    // Go type: time
	    at: any;
	    durationMs: number;
	    rows: number;
	    isQuery: boolean;
	    error: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profile = source["profile"];
	        this.database = source["database"];
	        this.sql = source["sql"];
	        this.at = this.convertValues(source["at"], null);
	        this.durationMs = source["durationMs"];
	        this.rows = source["rows"];
	        this.isQuery = source["isQuery"];
	        this.error = source["error"];
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
	export class Saved {
	    id: string;
	    name: string;
	    sql: string;
	    driver: string;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Saved(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sql = source["sql"];
	        this.driver = source["driver"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class CellPreview {
	    size: number;
	    mime: string;
	    kind: string;
	    offset: number;
	    image: string;
	    text: string;
	    hex: string;
	
	    static createFrom(source: any = {}) {
	        return new CellPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.size = source["size"];
	        this.mime = source["mime"];
	        this.kind = source["kind"];
	        this.offset = source["offset"];
	        this.image = source["image"];
	        this.text = source["text"];
	        this.hex = source["hex"];
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
	export class CellRequest {
	    table: TableRef;
	    column: string;
	    key: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new CellRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = this.convertValues(source["table"], TableRef);
	        this.column = source["column"];
	        this.key = source["key"];
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
	export class ColumnDef {
	    name: string;
	    type: string;
	    nullable: boolean;
	    default: string;
	    primaryKey: boolean;
	    autoIncrement: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.nullable = source["nullable"];
	        this.default = source["default"];
	        this.primaryKey = source["primaryKey"];
	        this.autoIncrement = source["autoIncrement"];
	    }
	}
	export class ExportRequest {
	    profile: string;
	    database: string;
	    kind: string;
	    sql: string;
	    schema: string;
	    table: string;
	    filter: string;
	    orderBy: string;
	    desc: boolean;
	    format: string;
	    path: string;
	    delimiter: string;
	    header: boolean;
	    bom: boolean;
	    sqlTable: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.database = source["database"];
	        this.kind = source["kind"];
	        this.sql = source["sql"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.filter = source["filter"];
	        this.orderBy = source["orderBy"];
	        this.desc = source["desc"];
	        this.format = source["format"];
	        this.path = source["path"];
	        this.delimiter = source["delimiter"];
	        this.header = source["header"];
	        this.bom = source["bom"];
	        this.sqlTable = source["sqlTable"];
	    }
	}
	export class ExportResult {
	    path: string;
	    rows: number;
	    bytes: number;
	    durationMs: number;
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.rows = source["rows"];
	        this.bytes = source["bytes"];
	        this.durationMs = source["durationMs"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class FKDef {
	    name: string;
	    columns: string[];
	    refSchema: string;
	    refTable: string;
	    refColumns: string[];
	    onDelete: string;
	    onUpdate: string;
	
	    static createFrom(source: any = {}) {
	        return new FKDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.refSchema = source["refSchema"];
	        this.refTable = source["refTable"];
	        this.refColumns = source["refColumns"];
	        this.onDelete = source["onDelete"];
	        this.onUpdate = source["onUpdate"];
	    }
	}
	export class ImportMap {
	    column: string;
	    source: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportMap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.source = source["source"];
	    }
	}
	export class ImportRequest {
	    table: TableRef;
	    path: string;
	    delimiter: string;
	    hasHeader: boolean;
	    mapping: ImportMap[];
	    emptyAsNull: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = this.convertValues(source["table"], TableRef);
	        this.path = source["path"];
	        this.delimiter = source["delimiter"];
	        this.hasHeader = source["hasHeader"];
	        this.mapping = this.convertValues(source["mapping"], ImportMap);
	        this.emptyAsNull = source["emptyAsNull"];
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
	export class ImportResult {
	    inserted: number;
	    durationMs: number;
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inserted = source["inserted"];
	        this.durationMs = source["durationMs"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class IndexColumn {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new IndexColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class IndexDef {
	    name: string;
	    columns: string[];
	    unique: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IndexDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.unique = source["unique"];
	    }
	}
	export class IndexTable {
	    schema: string;
	    name: string;
	    isView: boolean;
	    columns: IndexColumn[];
	
	    static createFrom(source: any = {}) {
	        return new IndexTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.isView = source["isView"];
	        this.columns = this.convertValues(source["columns"], IndexColumn);
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
	export class SQLFile {
	    path: string;
	    name: string;
	    content: string;
	    crlf: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SQLFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.crlf = source["crlf"];
	    }
	}
	export class SchemaIndex {
	    tables: IndexTable[];
	    defaultSchema: string;
	
	    static createFrom(source: any = {}) {
	        return new SchemaIndex(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tables = this.convertValues(source["tables"], IndexTable);
	        this.defaultSchema = source["defaultSchema"];
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
	export class SchemaOp {
	    kind: string;
	    target: string;
	    newName: string;
	    column?: ColumnDef;
	    index?: IndexDef;
	    fk?: FKDef;
	    columns: ColumnDef[];
	
	    static createFrom(source: any = {}) {
	        return new SchemaOp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.target = source["target"];
	        this.newName = source["newName"];
	        this.column = this.convertValues(source["column"], ColumnDef);
	        this.index = this.convertValues(source["index"], IndexDef);
	        this.fk = this.convertValues(source["fk"], FKDef);
	        this.columns = this.convertValues(source["columns"], ColumnDef);
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
	export class SchemaPlan {
	    statements: string[];
	    warnings: string[];
	    transactional: boolean;
	    destructive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SchemaPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statements = source["statements"];
	        this.warnings = source["warnings"];
	        this.transactional = source["transactional"];
	        this.destructive = source["destructive"];
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

export namespace transfer {
	
	export class Preview {
	    delimiter: string;
	    header: string[];
	    rows: string[][];
	    columns: number;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.delimiter = source["delimiter"];
	        this.header = source["header"];
	        this.rows = source["rows"];
	        this.columns = source["columns"];
	        this.warnings = source["warnings"];
	    }
	}

}

