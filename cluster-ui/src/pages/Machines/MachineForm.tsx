import React from 'react'
import {
  Form,
  Input,
  Checkbox,
  Button,
  Collapse,
  Space,
  Select,
  Typography,
} from 'antd'

import {
  IGlobalLoginOptions,
  DEF_UESRNAME,
  Machine,
  MachineMap,
  DEF_SSH_PORT,
} from '_types'

function correctFormValues(values: any): any {
  let newValues = {} as any
  for (const key of Object.keys(values)) {
    let v = values[key]
    newValues[key] = v

    if (v === undefined) {
      continue
    }
    if (typeof v !== 'string') {
      continue
    }
    v = v.trim()
    if (v === '') {
      v = undefined
    }
    if (key === 'ssh_port' && v !== undefined) {
      v = parseInt(v)
      if (v <= 0) {
        v = undefined
      }
    }
    newValues[key] = v
  }
  return newValues
}

interface MachineFormProps {
  globalLoginOptions: IGlobalLoginOptions
  machine?: Machine
  machines: MachineMap
  onAdd?: (machine: Machine, close: boolean) => boolean
  onUpdate?: (machine: Machine) => boolean
}

const defMachine = new Machine()

export default function MachineForm({
  globalLoginOptions,
  machine,
  machines,
  onAdd,
  onUpdate,
}: MachineFormProps) {
  const [form] = Form.useForm()

  const addNew: boolean = machine === undefined

  function onReset() {
    form.resetFields()
  }

  function onAddAndNext() {
    form
      .validateFields()
      .then((values) => {
        correctFormValues(values)
        let m = Machine.deSerial(values)
        const ok = onAdd && onAdd(m, false)
        if (ok) {
          form.resetFields()
        }
      })
      .catch((_err) => {
        console.log(_err)
      })
  }

  function handleFinish(values: any) {
    let newValues
    if (addNew) {
      newValues = correctFormValues(values)
    } else {
      newValues = correctFormValues({ ...machine, ...values })
    }

    let m = Machine.deSerial(newValues)
    if (addNew) {
      onAdd && onAdd(m, true)
    } else {
      m.id = machine!.id
      onUpdate && onUpdate(m)
    }
  }

  function handleTemplateChange(machineID: any) {
    if (machineID === undefined) {
      return
    }

    const m = machines[machineID]
    form.setFieldsValue({
      ...m,
      name: undefined,
    })
  }

  return (
    <Form
      form={form}
      onFinish={handleFinish}
      layout="vertical"
      initialValues={addNew ? defMachine : machine}
    >
      <Collapse defaultActiveKey={['import', 'basic']} bordered={false}>
        {addNew && (
          <Collapse.Panel key="import" header="导入模板">
            <Form.Item label="以现有主机配置作为模板">
              <Select
                allowClear
                placeholder="选择模板"
                onChange={handleTemplateChange}
                showSearch
                filterOption={(input, option) => {
                  const m = machines[option?.key!]
                  const v = `${m.name} ${m.userName(
                    globalLoginOptions
                  )}@${m.address()}`
                  return v.indexOf(input) > -1
                }}
              >
                {Object.values(machines).map((m) => {
                  return (
                    <Select.Option value={m.id} key={m.id}>
                      <div>{m.fullMachineName(globalLoginOptions)}</div>
                      <div>
                        <Typography.Text type="secondary">
                          {m.userName(globalLoginOptions)}@{m.address()}
                        </Typography.Text>
                      </div>
                    </Select.Option>
                  )
                })}
              </Select>
            </Form.Item>
          </Collapse.Panel>
        )}

        <Collapse.Panel key="basic" header="详细配置">
          <Form.Item label="唯一名称" name="name">
            <Input placeholder="留空自动生成" />
          </Form.Item>
          <Form.Item
            label="地址"
            name="host"
            rules={[
              {
                required: true,
                message: '请输入主机地址',
              },
            ]}
          >
            <Input placeholder="主机地址，IP 或域名" />
          </Form.Item>
          <Form.Item label="SSH 端口" name="ssh_port">
            <Input placeholder={DEF_SSH_PORT + ''} />
          </Form.Item>

          {false && (
            <>
              <Form.Item label="登录用户名" name="username">
                <Input
                  placeholder={globalLoginOptions.username || DEF_UESRNAME}
                  autoComplete="new-password"
                />
              </Form.Item>
              <Form.Item name="isPubKeyAuth" valuePropName="checked">
                <Checkbox>使用私钥登录</Checkbox>
              </Form.Item>
              <Form.Item
                noStyle
                shouldUpdate={(prevValues, currentValues) =>
                  prevValues.isPubKeyAuth !== currentValues.isPubKeyAuth
                }
              >
                {({ getFieldValue }) => {
                  return getFieldValue('isPubKeyAuth') ? (
                    <>
                      <Form.Item label="私钥" name="privateKey">
                        <Input.TextArea
                          placeholder={globalLoginOptions.privateKey}
                        />
                      </Form.Item>
                      <Form.Item label="私钥密码" name="privateKeyPassword">
                        <Input.Password
                          placeholder={globalLoginOptions.privateKeyPassword}
                          autoComplete="new-password"
                        />
                      </Form.Item>
                    </>
                  ) : (
                    <Form.Item label="密码" name="password">
                      <Input.Password
                        placeholder={globalLoginOptions.password}
                        autoComplete="new-password"
                      />
                    </Form.Item>
                  )
                }}
              </Form.Item>
            </>
          )}
        </Collapse.Panel>

        <Collapse.Panel key="advance" header="高级配置">
          <Form.Item label="位置标签: DC" name="dc">
            <Input />
          </Form.Item>
          <Form.Item label="位置标签: Rack" name="rack">
            <Input />
          </Form.Item>
        </Collapse.Panel>

        <Form.Item style={{ padding: 16 }}>
          <Space>
            <Button type="primary" htmlType="submit">
              {addNew ? '添加' : '更新'}
            </Button>
            {addNew && (
              <Button htmlType="button" onClick={onAddAndNext}>
                保存并添加下一步
              </Button>
            )}
            <Button htmlType="button" onClick={onReset}>
              重置
            </Button>
          </Space>
        </Form.Item>
      </Collapse>
    </Form>
  )
}
