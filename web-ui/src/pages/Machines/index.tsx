import React, { useState, useCallback } from 'react'
import { Button, Drawer, Modal, Space } from 'antd'

import { Root } from '_components'
import { useMachines, useComps, useGlobalLoginOptions } from '_hooks'
import { Machine } from '_types'

import MachineForm from './MachineForm'
import MachinesTable from './MachinesTable'
import GlobalLoginOptionsForm from './GlobalLoginOptionsForm'

export default function MachinesPage() {
  const [showForm, setShowForm] = useState(false)
  const [curMachine, setCurMachine] = useState<Machine | undefined>(undefined)

  const { machines, setMachineObjs } = useMachines()
  const { comps, setCompObjs } = useComps()
  const { globalLoginOptions, setGlobalLoginOptions } = useGlobalLoginOptions()

  function addOrUpdateMachine(machine: Machine): boolean {
    let dup = Object.values(machines).find((m) => m.host === machine.host)
    if (dup && dup.id !== machine.id) {
      Modal.error({
        title: '添加失败',
        content: `该主机 ${machine.host} 已存在`,
      })
      return false
    }

    dup = Object.values(machines).find(
      (m) =>
        m.fullName(globalLoginOptions) === machine.fullName(globalLoginOptions)
    )
    if (dup && dup.id !== machine.id) {
      Modal.error({
        title: '添加失败',
        content: `该主机 name ${machine.name} 已被使用`,
      })
      return false
    }

    setMachineObjs({
      ...machines,
      [machine.id]: machine,
    })
    return true
  }

  function handleAddMachine(machine: Machine, close: boolean): boolean {
    let ret = addOrUpdateMachine(machine)
    if (!ret) {
      return false
    }

    if (close) {
      setShowForm(false)
    }
    return true
  }

  function handleUpdateMachine(machine: Machine): boolean {
    let ret = addOrUpdateMachine(machine)
    if (!ret) {
      return false
    }

    setShowForm(false)
    return true
  }

  const addMachine = useCallback(() => {
    setCurMachine(undefined)
    setShowForm(true)
  }, [])

  const editMachine = useCallback((m: Machine) => {
    setCurMachine(m)
    setShowForm(true)
  }, [])

  const deleteMachine = useCallback(
    (m: Machine) => {
      const newMachines = { ...machines }
      delete newMachines[m.id]
      setMachineObjs(newMachines)

      // delete related component
      const newComps = { ...comps }
      const belongedComps = Object.values(comps).filter(
        (c) => c.machineID === m.id
      )
      for (const c of belongedComps) {
        delete newComps[c.id]
      }
      setCompObjs(newComps)
    },
    [machines, setMachineObjs, comps, setCompObjs]
  )

  return (
    <Root>
      <div style={{ border: '1px solid #ccc', padding: 16, marginBottom: 16 }}>
        <h2>全局默认登录选项：</h2>
        <GlobalLoginOptionsForm
          globalLoginOptions={globalLoginOptions}
          onUpdateGlobalLoginOptions={setGlobalLoginOptions}
        />
      </div>

      <Space>
        <Button type="primary" onClick={addMachine}>
          添加机器
        </Button>
      </Space>

      <div style={{ marginTop: 16 }}>
        <MachinesTable
          globalLoginOptions={globalLoginOptions}
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
          globalLoginOptions={globalLoginOptions}
          machines={machines}
          machine={curMachine}
          onAdd={handleAddMachine}
          onUpdate={handleUpdateMachine}
        />
      </Drawer>
    </Root>
  )
}
