<template>
  <div>
    <div class="full-width" ref="container"></div>
    <q-dialog v-model="showTaskDialog">
      <q-card v-if="selectedTask">
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">
            Task {{ selectedTask.name }}
          </div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section>
          <q-markup-table>
            <tbody>
              <tr>
                <th class="text-left">Id</th>
                <td class="text-right">{{ selectedTask.id }}</td>
              </tr>
              <tr>
                <th class="text-left">Name</th>
                <td class="text-right">{{ selectedTask.name }}</td>
              </tr>
              <tr>
                <th class="text-left">Worker</th>
                <td class="text-right">{{ selectedTask.worker }}</td>
              </tr>
              <tr>
                <th class="text-left">Paused</th>
                <td class="text-right">{{ selectedTask.isPaused ? 'Yes' : 'No' }}</td>
              </tr>
              <tr>
                <th class="text-left">Complete</th>
                <td class="text-right">
                  <span v-if="selectedTask.busyExecuting">
                    <q-chip size="md" color="purple" text-color="white" icon="pending">
                      Executing ({{ selectedTask.remainingAttempts }})
                    </q-chip>
                  </span>
                  <span v-else>
                    <q-chip v-if="selectedTask.isComplete" size="md" color="green" text-color="white" icon="done">
                      Yes
                    </q-chip>
                    <q-chip v-if="!selectedTask.isComplete && (!selectedTask.errors ||  selectedTask.errors.length === 0)" size="md" color="blue" text-color="white" icon="hourglass_empty">
                      No ({{ selectedTask.remainingAttempts }})
                    </q-chip>
                    <q-chip v-if="!selectedTask.isComplete && selectedTask.errors && selectedTask.errors.length > 0" size="md" color="orange" text-color="white" icon="warning">
                      No ({{ selectedTask.remainingAttempts }})
                    </q-chip>
                  </span>
                </td>
              </tr>
              <tr>
                <th class="text-left">Actions</th>
                <td>
                  <CreateTaskModalButton label="" size="sm" flat :task-group="props.taskGroup" :parent-id="selectedTask.id" @on-create="onCreate">
                    <q-tooltip>
                      Add a child to this task
                    </q-tooltip>
                  </CreateTaskModalButton>
                  <EditTaskModalButton label="" size="sm" flat :task-group="props.taskGroup" :task="selectedTask" @on-update="onUpdate" />
                  <DeleteTaskModalButton label="" size="sm" flat :task="selectedTask" @on-delete="onDelete" />

                  <ResetTaskModalButton :task="selectedTask" @on-reset="(evt: any) => onReset(evt, selectedTask!)" />
                  <RetryTaskModalButton :task="selectedTask" @on-retry="(evt: any) => onRetry(evt, selectedTask!)" />

                  <q-btn v-if="!selectedTask.isComplete && !selectedTask.isPaused" flat icon="pause" color="primary" size="sm" @click="pauseTask(selectedTask!)">
                    <q-tooltip>
                      Pause this task
                    </q-tooltip>
                  </q-btn>
                  <q-btn v-if="!selectedTask.isComplete && selectedTask.isPaused" flat icon="play_arrow" color="primary" size="sm" @click="resumeTask(selectedTask!)">
                    <q-tooltip>
                      Resume this task
                    </q-tooltip>
                  </q-btn>
                </td>
              </tr>
            </tbody>
          </q-markup-table>
        </q-card-section>

        <q-card-section>
          <div class="text-h6">
            Task Output
          </div>
          <div>
            {{ selectedTask.output }}
          </div>
        </q-card-section>

        <q-card-section v-if="selectedTask.errors">
          <div class="text-h6">
            Task Errors
          </div>
          <div>
            {{ selectedTask.errors }}
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>
</div>
</template>

<script setup lang="ts">
import { onMounted, ref, reactive } from 'vue'
import _ from 'lodash'
import notifyError from 'src/lib/notifyError'
import ForceGraph, { LinkObject, NodeObject } from 'force-graph'
import { useTaskStore, Task } from 'src/stores/task-store'
import { TaskGroup } from 'src/stores/task-group-store'
import CreateTaskModalButton from 'src/components/task/CreateTaskModalButton.vue'
import EditTaskModalButton from 'src/components/task/EditTaskModalButton.vue'
import DeleteTaskModalButton from 'src/components/task/DeleteTaskModalButton.vue'
import ResetTaskModalButton from 'src/components/task/ResetTaskModalButton.vue'
import RetryTaskModalButton from 'src/components/task/RetryTaskModalButton.vue'

const taskStore = useTaskStore()

interface Props {
  taskGroup: TaskGroup;
}
const props = defineProps<Props>()

const loading = ref(true)
const showTaskDialog = ref(false)
const tasks: { [id: string]: Task } = reactive({})
const container = ref<HTMLElement>()
const selectedTask = ref<Task | null>(null)

function onCreate (task: Task) {
  loadTasks()
}

function onUpdate (task: Task) {
  loadTasks()
}

function onDelete (taskId: string) {
  loadTasks()
}

async function pauseTask (task: Task) {
  try {
    task.pauseWait = true
    await taskStore.pauseTask(props.taskGroup.id, task.id)
    task.isPaused = true
  } catch (e) {
    notifyError(e)
  } finally {
    task.pauseWait = false
  }
}

async function resumeTask (task: Task) {
  try {
    task.pauseWait = true
    await taskStore.resumeTask(props.taskGroup.id, task.id)
    task.isPaused = false
  } catch (e) {
    notifyError(e)
  } finally {
    task.pauseWait = false
  }
}

function onReset (evt: any, task: Task) {
  Object.assign(task, evt)
  throttledRenderGraph()
}

function onRetry (evt: any, task: Task) {
  Object.assign(task, evt)
  throttledRenderGraph()
}

const NODE_REL_SIZE = 4
const graph = ForceGraph()
graph.dagMode('td')
  .dagLevelDistance(40)
  .nodeRelSize(NODE_REL_SIZE)
  // .backgroundColor('#101020')
  .linkColor(() => 'rgba(0,0,0,0.8)')
  .nodeLabel((node) => {
    if (node.id) {
      return tasks[node.id].name
    }
    return '?'
  })
  .nodeColor((node) => {
    if (node.id) {
      const task = tasks[node.id]
      if (task) {
        if (task.busyExecuting) {
          return 'rgba(0,0,255,0.8)'
        }
        if (task.isComplete) {
          return 'rgba(0,255,0,0.8)'
        }
        if (task.errors && task.errors.length > 0) {
          return 'rgba(255,240,0,0.8)'
        }
        return 'rgba(196,196,196,0.8)'
      }
    }
    return 'rgba(0,0,0,0.8)'
  })
  .onNodeClick((node) => {
    if (node.id) {
      const task = tasks[node.id]
      if (task) {
        selectedTask.value = task
        showTaskDialog.value = true
      }
    }
  })
  // .linkDirectionalParticles(2)
  // .linkDirectionalParticleWidth(2)
  // .d3Force('collision', d3.forceCollide(node => Math.sqrt(100 / (node.level + 1)) * NODE_REL_SIZE))

const throttledRenderGraph = _.throttle(renderGraph, 1000)

function renderGraph () {
  let nodes : Array<NodeObject> = []
  let links : Array<LinkObject> = []

  const snapshot = JSON.parse(JSON.stringify(tasks))

  for (const task of _.values(snapshot)) {
    if (!task.node) {
      task.node = {
        id: task.id
      }
    }
    nodes.push(task.node)
  }

  for (const task of _.values(snapshot)) {
    if (task.parentIds) {
      for (const parentId of task.parentIds) {
        if (snapshot[parentId]) {
          links.push({
            source: parentId,
            target: task.id
          })
        }
      }
    }
  }

  if (container.value) {
    nodes = _.compact(nodes)
    links = _.compact(links)
    graph(container.value).graphData({ nodes, links })
  }
}

async function loadTasks () {
  try {
    loading.value = true
    // Load all tasks
    const result = await taskStore.getTasks(props.taskGroup.id, 1, 0)
    for (const task of result.tasks) {
      task.pauseWait = false
      task.resumeWait = false
      task.resetWait = false
      task.retryWait = false

      // https://github.com/graphology/graphology/blob/master/docs/index.md
      // https://codesandbox.io/s/github/jacomyal/sigma.js/tree/main/examples/template

      task.node = {
        id: task.id
      }
      tasks[task.id] = task
      throttledRenderGraph()
    }
  } catch (e) {
    notifyError(e)
  } finally {
    loading.value = false
  }
}

function taskUpdated (update: Task) {
  Object.assign(tasks[update.id], update)
  throttledRenderGraph()
}

function taskDeleted (deleted: Task) {
  delete tasks[deleted.id]
  throttledRenderGraph()
}

function taskCreated (task: Task) {
  tasks[task.id] = task
  throttledRenderGraph()
}

defineExpose({
  loadTasks,
  taskUpdated,
  taskDeleted,
  taskCreated
})

onMounted(async () => {
  await loadTasks()
})
</script>
