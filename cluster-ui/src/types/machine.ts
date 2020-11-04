import uniqid from 'uniqid'

export const DEF_UESRNAME = 'root'
export interface IGlobalLoginOptions {
  username?: string
  password?: string
  privateKey?: string
  privateKeyPassword?: string
}

//////////////////////////

export type MachineMap = Record<string, Machine>
export type Arch = 'amd64' | 'arm64'

export const DEF_SSH_PORT = 22
export class Machine {
  id: string
  host: string = ''
  ssh_port?: number
  name?: string
  arch: Arch = 'amd64'

  isPubKeyAuth: boolean = true
  privateKey?: string
  privateKeyPassword?: string

  username?: string
  password?: string

  zone?: string
  dc?: string

  constructor() {
    this.id = uniqid()
  }

  static deSerial(obj: any): Machine {
    let m = new Machine()
    Object.assign(m, obj)
    return m
  }

  public userName(globalLoginOptions: IGlobalLoginOptions) {
    return `${this.username || globalLoginOptions.username || DEF_UESRNAME}`
  }

  public port() {
    return this.ssh_port || DEF_SSH_PORT
  }

  public address() {
    return `${this.host}:${this.port()}`
  }

  public fullMachineName(globalLoginOptions: IGlobalLoginOptions): string {
    if (this.name) {
      return this.name
    }
    return `${this.userName(globalLoginOptions)}@${this.host}`
  }
}
