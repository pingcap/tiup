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
import uniqid from 'uniqid'

export interface IMachine {
  id: string
  name: string
  host: string
  ssh_port?: number

  isPubKeyAuth: boolean
  privateKey: string
  privateKeyPassword: string

  username: string
  password: string

  dc: string
  rack: string
}
export const DEF_SSH_PORT = 22

const defMachine: IMachine = {
  id: '',
  name: '',
  host: '',
  ssh_port: undefined,

  isPubKeyAuth: false,
  privateKey: '',
  privateKeyPassword: '',

  username: '',
  password: '',

  dc: '',
  rack: '',
}

function correctFormValues(values: any) {
  for (const key of Object.keys(values)) {
    if (key !== 'ssh_port' && key !== 'isPubKeyAuth') {
      values[key] = values[key].trim()
    }
    if (key === 'ssh_port' && values[key] !== undefined) {
      values[key] = parseInt(values[key])
      if (values[key] === 0) {
        values[key] = undefined
      }
    }
  }
  if (values.name === '') {
    values.name = `${values.username}@${values.host}:${
      values.ssh_port || DEF_SSH_PORT
    }`
  }
}

interface IMachineFormProps {
  machine?: IMachine
  machines: { [key: string]: IMachine }
  onAdd?: (machine: IMachine, close: boolean) => boolean
  onUpdate?: (machine: IMachine) => boolean
}

export default function MachineForm({
  machine,
  machines,
  onAdd,
  onUpdate,
}: IMachineFormProps) {
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
        const ok =
          onAdd &&
          onAdd(
            {
              ...defMachine,
              ...(values as IMachine),
              id: uniqid(),
            },
            false
          )
        if (ok) {
          form.resetFields()
        }
      })
      .catch((_err) => {})
  }

  function handleFinish(values: any) {
    correctFormValues(values)
    if (addNew) {
      onAdd &&
        onAdd(
          {
            ...defMachine,
            ...(values as IMachine),
            id: uniqid(),
          },
          true
        )
    } else {
      onUpdate &&
        onUpdate({
          ...defMachine,
          ...(values as IMachine),
          id: machine!.id,
        })
    }
  }

  function handleTemplateChange(machineID: any) {
    if (machineID === undefined) {
      return
    }

    const m = machines[machineID]
    form.setFieldsValue({
      ssh_port: m.ssh_port,

      isPubKeyAuth: m.isPubKeyAuth,
      privateKey: m.privateKey,
      privateKeyPassword: m.privateKeyPassword,

      username: m.username,
      password: m.password,

      dc: m.dc,
      rack: m.rack,
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
                  const v = `${m.name} ${m.username}@${m.host}:${m.ssh_port}`
                  return v.indexOf(input) > -1
                }}
                optionLabelProp="label"
              >
                {Object.values(machines).map((m) => {
                  return (
                    <Select.Option value={m.id} label={m.name} key={m.id}>
                      <div>{m.name}</div>
                      <div>
                        <Typography.Text type="secondary">
                          {m.username}@{m.host}:{m.ssh_port}
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
          <Form.Item
            label="登录用户名"
            name="username"
            rules={[
              {
                required: true,
                message: '请输入登录用户名',
              },
            ]}
          >
            <Input />
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
                  <Form.Item
                    label="私钥"
                    name="privateKey"
                    rules={[
                      {
                        required: true,
                        message: '请输入私钥',
                      },
                    ]}
                  >
                    <Input.TextArea />
                  </Form.Item>
                  <Form.Item label="私钥密码" name="privateKeyPassword">
                    <Input.Password />
                  </Form.Item>
                </>
              ) : (
                <Form.Item
                  label="密码"
                  name="password"
                  rules={[
                    {
                      required: true,
                      message: '请输入登录密码',
                    },
                  ]}
                >
                  <Input.Password />
                </Form.Item>
              )
            }}
          </Form.Item>
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
