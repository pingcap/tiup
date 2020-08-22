import React, { useState, useCallback } from 'react'
import { Button, Drawer, Modal, Space } from 'antd'
import { useLocalStorageState } from 'ahooks'

import MachineForm, { IMachine } from './MachineForm'
import MachinesTable from './MachinesTable'
import { IComponent } from '../Deployment/DeploymentTable'

export default function MachinesPage() {
  const [showForm, setShowForm] = useState(false)
  const [curMachine, setCurMachine] = useState<IMachine | undefined>(undefined)

  const [machines, setMachines] = useLocalStorageState<{
    [key: string]: IMachine
  }>('machines', {})
  const [components, setComponents] = useLocalStorageState<{
    [key: string]: IComponent
  }>('components', {})

  function handleAddMachine(machine: IMachine, close: boolean) {
    let dup = Object.values(machines).find((m) => m.host === machine.host)
    if (dup !== undefined) {
      Modal.error({
        title: '添加失败',
        content: `该主机 ${machine.host} 已存在`,
      })
      return false
    }

    dup = Object.values(machines).find((m) => m.name === machine.name)
    if (dup !== undefined) {
      Modal.error({
        title: '添加失败',
        content: `该主机 name ${machine.name} 已被使用`,
      })
      return false
    }

    setMachines({ ...machines, [machine.id]: machine })
    if (close) {
      setShowForm(false)
    }
    return true
  }

  function handleUpdateMachine(machine: IMachine) {
    // TODO: duplicated with above code
    let dup = Object.values(machines).find((m) => m.host === machine.host)
    if (dup && dup.id !== machine.id) {
      Modal.error({
        title: '添加失败',
        content: `该主机 ${machine.host} 已存在`,
      })
      return false
    }

    dup = Object.values(machines).find((m) => m.name === machine.name)
    if (dup && dup.id !== machine.id) {
      Modal.error({
        title: '添加失败',
        content: `该主机 name ${machine.name} 已被使用`,
      })
      return false
    }
    setMachines({
      ...machines,
      [machine.id]: machine,
    })
    setShowForm(false)
    return true
  }

  const addMachine = useCallback(() => {
    setCurMachine(undefined)
    setShowForm(true)
  }, [])

  const editMachine = useCallback((m: IMachine) => {
    setCurMachine(m)
    setShowForm(true)
  }, [])

  const deleteMachine = useCallback(
    (m: IMachine) => {
      const newMachines = { ...machines }
      delete newMachines[m.id]
      setMachines(newMachines)

      // delete related component
      const newComps = { ...components }
      const belongedComps = Object.values(components).filter(
        (c) => c.machineID === m.id
      )
      for (const c of belongedComps) {
        delete newComps[c.id]
      }
      setComponents(newComps)
    },
    [machines, setMachines, components, setComponents]
  )

  return (
    <div>
      <Space>
        <Button type="primary" onClick={addMachine}>
          添加机器
        </Button>
      </Space>

      <div style={{ marginTop: 16 }}>
        <MachinesTable
          machines={machines}
          onEdit={editMachine}
          onDelete={deleteMachine}
        />
      </div>

      <Drawer
        title={curMachine ? '修改 SSH 远程主机' : '添加 SSH 远程主机'}
        width={400}
        bodyStyle={{ padding: 0 }}
        closable={true}
        visible={showForm}
        onClose={() => setShowForm(false)}
        destroyOnClose={true}
      >
        <MachineForm
          machines={machines}
          machine={curMachine}
          onAdd={handleAddMachine}
          onUpdate={handleUpdateMachine}
        />
      </Drawer>
    </div>
  )
}
