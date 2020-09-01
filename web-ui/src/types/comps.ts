import uniqid from 'uniqid'

const COMP_COLORS = {
  TiDB: 'magenta',
  TiKV: 'orange',
  TiFlash: 'red',
  PD: 'purple',
  Prometheus: 'green',
  Grafana: 'cyan',
  AlertManager: 'blue',
}

export type CompTypes = keyof typeof COMP_COLORS // "TiDB" | "TiKV" | "TiFlash" | ...

export const COMP_TYPES_ARR = [
  'TiDB',
  'TiKV',
  'TiFlash',
  'PD',
  'Prometheus',
  'Grafana',
  'AlertManager',
]

export type CompMap = {
  [key: string]: BaseComp
}

///////////////////////

export const DEF_DEPLOY_DIR_PREFIX = 'tidb-deploy'
export const DEF_DATA_DIR_PREFIX = 'tidb-data'
export abstract class BaseComp {
  id: string
  machineID: string
  type: CompTypes
  for_scale_out: boolean
  priority: number
  color: string

  deploy_dir_prefix?: string // "tidb-deploy"
  data_dir_prefix?: string // "tidb-data", TiDB and Grafana have no data_dir

  constructor(machineID: string, compType: CompTypes, for_scale_out: boolean) {
    this.id = uniqid()
    this.machineID = machineID
    this.type = compType
    this.for_scale_out = for_scale_out
    this.priority = COMP_TYPES_ARR.indexOf(compType)
    this.color = COMP_COLORS[compType]
  }

  public name(): string {
    return this.type.toLowerCase()
  }

  public copyPaths(comp: BaseComp): void {
    this.deploy_dir_prefix = comp.deploy_dir_prefix
    this.data_dir_prefix = comp.data_dir_prefix
  }

  public deployPathPrefix() {
    return this.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX
  }

  public dataPathPrefix() {
    return this.data_dir_prefix || DEF_DATA_DIR_PREFIX
  }

  public allPathsPrefix() {
    if (this.dataPathPrefix()) {
      return `${this.deployPathPrefix()},${this.dataPathPrefix()}`
    }
    return this.deployPathPrefix()
  }

  public deployPathFull(): string {
    return `${this.deployPathPrefix()}/${this.name()}-${this.symbolPort()}`
  }

  public dataPathFull(): string {
    if (this.dataPathPrefix().startsWith('/')) {
      return `${this.dataPathPrefix()}/${this.name()}-${this.symbolPort()}`
    }
    return `${this.deployPathFull()}/${this.dataPathPrefix()}`
  }

  public abstract symbolPort(): number
  public abstract ports(): string
  public abstract increasePorts(comp: BaseComp): void

  static create(
    compType: CompTypes,
    machineID: string,
    for_scale_out: boolean
  ): BaseComp {
    switch (compType) {
      case 'TiDB':
        return new TiDBComp(machineID, for_scale_out)
      case 'TiKV':
        return new TiKVComp(machineID, for_scale_out)
      case 'TiFlash':
        return new TiFlashComp(machineID, for_scale_out)
      case 'PD':
        return new PDComp(machineID, for_scale_out)
      case 'Prometheus':
        return new PromComp(machineID, for_scale_out)
      case 'Grafana':
        return new GrafanaComp(machineID, for_scale_out)
      case 'AlertManager':
        return new AlertManagerComp(machineID, for_scale_out)
    }
  }

  static deSerial(obj: any): BaseComp {
    let comp = this.create(obj.type, obj.machineID, obj.for_scale_out)
    Object.assign(comp, obj)
    return comp
  }
}

///////////////////////

export const DEF_TIDB_PORT = 4000
export const DEF_TIDB_STATUS_PORT = 10080
export class TiDBComp extends BaseComp {
  port?: number
  status_port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'TiDB', for_scale_out)
  }

  public dataPathPrefix() {
    return ''
  }

  public symbolPort() {
    return this.port || DEF_TIDB_PORT
  }

  public ports() {
    return `${this.port || DEF_TIDB_PORT}/${
      this.status_port || DEF_TIDB_STATUS_PORT
    }`
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as TiDBComp
    this.port = (c.port || DEF_TIDB_PORT) + 1
    this.status_port = (c.status_port || DEF_TIDB_STATUS_PORT) + 1
  }
}

///////////////////////

export const DEF_TIKV_PORT = 20160
export const DEF_TIKV_STATUS_PORT = 20180
export class TiKVComp extends BaseComp {
  port?: number
  status_port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'TiKV', for_scale_out)
  }

  public symbolPort() {
    return this.port || DEF_TIKV_PORT
  }

  public ports() {
    return `${this.port || DEF_TIKV_PORT}/${
      this.status_port || DEF_TIKV_STATUS_PORT
    }`
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as TiKVComp
    this.port = (c.port || DEF_TIKV_PORT) + 1
    this.status_port = (c.status_port || DEF_TIKV_STATUS_PORT) + 1
  }
}

///////////////////////

export const DEF_TIFLASH_TCP_PORT = 9000
export const DEF_TIFLASH_HTTP_PORT = 8123
export const DEF_TIFLASH_SERVICE_PORT = 3930
export const DEF_TIFLASH_PROXY_PORT = 20170
export const DEF_TIFLASH_PROXY_STATUS_PORT = 20292
export const DEF_TIFLASH_METRICS_PORT = 8234
export class TiFlashComp extends BaseComp {
  tcp_port?: number
  http_port?: number
  flash_service_port?: number
  flash_proxy_port?: number
  flash_proxy_status_port?: number
  metrics_port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'TiFlash', for_scale_out)
  }

  public symbolPort() {
    return this.tcp_port || DEF_TIFLASH_TCP_PORT
  }

  public ports() {
    return `${this.tcp_port || DEF_TIFLASH_TCP_PORT}/${
      this.http_port || DEF_TIFLASH_HTTP_PORT
    }/${this.flash_service_port || DEF_TIFLASH_SERVICE_PORT}/${
      this.flash_proxy_port || DEF_TIFLASH_PROXY_PORT
    }/${this.flash_proxy_status_port || DEF_TIFLASH_PROXY_STATUS_PORT}/${
      this.metrics_port || DEF_TIFLASH_METRICS_PORT
    }`
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as TiFlashComp
    this.tcp_port = (c.tcp_port || DEF_TIFLASH_TCP_PORT) + 1
    this.http_port = (c.http_port || DEF_TIFLASH_HTTP_PORT) + 1
    this.flash_service_port =
      (c.flash_service_port || DEF_TIFLASH_SERVICE_PORT) + 1
    this.flash_proxy_port = (c.flash_proxy_port || DEF_TIFLASH_PROXY_PORT) + 1
    this.flash_proxy_status_port =
      (c.flash_proxy_status_port || DEF_TIFLASH_PROXY_STATUS_PORT) + 1
    this.metrics_port = (c.metrics_port || DEF_TIFLASH_METRICS_PORT) + 1
  }
}

///////////////////////

export const DEF_PD_CLIENT_PORT = 2379
export const DEF_PD_PEER_PORT = 2380
export class PDComp extends BaseComp {
  client_port?: number
  peer_port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'PD', for_scale_out)
  }

  public symbolPort() {
    return this.client_port || DEF_PD_CLIENT_PORT
  }

  public ports() {
    return `${this.client_port || DEF_PD_CLIENT_PORT}/${
      this.peer_port || DEF_PD_PEER_PORT
    }`
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as PDComp
    this.client_port = (c.client_port || DEF_PD_CLIENT_PORT) + 1
    this.peer_port = (c.peer_port || DEF_PD_PEER_PORT) + 1
  }
}

///////////////////////

export const DEF_PROM_PORT = 9090
export class PromComp extends BaseComp {
  port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'Prometheus', for_scale_out)
  }

  public name(): string {
    return 'monitoring'
  }

  public symbolPort() {
    return this.port || DEF_PROM_PORT
  }

  public ports() {
    return `${this.port || DEF_PROM_PORT}`
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as PromComp
    this.port = (c.port || DEF_PROM_PORT) + 1
  }
}

///////////////////////

export const DEF_GRAFANA_PORT = 3000
export class GrafanaComp extends BaseComp {
  port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'Grafana', for_scale_out)
  }

  public dataPathPrefix() {
    return ''
  }

  public symbolPort() {
    return this.port || DEF_GRAFANA_PORT
  }

  public ports() {
    return `${this.port || DEF_GRAFANA_PORT}`
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as GrafanaComp
    this.port = (c.port || DEF_GRAFANA_PORT) + 1
  }
}

///////////////////////

export const DEF_ALERT_WEB_PORT = 9093
export const DEF_ALERT_CLUSTER_PORT = 9094
export class AlertManagerComp extends BaseComp {
  web_port?: number
  cluster_port?: number

  constructor(machineID: string, for_scale_out: boolean) {
    super(machineID, 'AlertManager', for_scale_out)
  }

  public symbolPort() {
    return this.web_port || DEF_ALERT_WEB_PORT
  }

  public ports() {
    return `${this.web_port || DEF_ALERT_WEB_PORT}/${
      this.cluster_port || DEF_ALERT_CLUSTER_PORT
    }
    `
  }

  public increasePorts(comp: BaseComp) {
    const c = comp as AlertManagerComp
    this.web_port = (c.web_port || DEF_ALERT_WEB_PORT) + 1
    this.cluster_port = (c.cluster_port || DEF_ALERT_CLUSTER_PORT) + 1
  }
}
