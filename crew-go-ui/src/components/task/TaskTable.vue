<template>
  <div class="full-width">
    <q-table
      :title="props.title"
      :rows="tasks"
      :columns="columns"
      v-model:pagination="paginationModel"
      row-key="id"
      :loading="loading"
      :flat="props.embedded"
      @request="onRequest"
      class="full-width"
      wrap-cells
    >
      <template v-slot:top>
        <div class="row full-width text-center" :class="{'text-left': !$q.screen.xs}">
          <div v-if="props.title" class="col-12 col-sm-3">
            <div class="q-table__title">
              {{ props.title }}
            </div>
          </div>

          <div :class="{
            'q-mt-sm': $q.screen.xs,
            'col-12 col-sm-5': props.title && !hideCreateButtons,
            'col-12 col-sm-9': props.title && hideCreateButtons,
            'col-12 col-sm-8': !props.title && !hideCreateButtons,
            'col-12': !props.title && props.hideCreateButtons
          }">
            <q-input
              type="text"
              v-model="search"
              placeholder="Search"
              filled
              dense
              @keyup.enter="loadTasks"
            >
              <template v-slot:after>
                <q-btn round dense flat icon="search" @click="loadTasks" />
                <q-btn round dense flat icon="clear" @click="clearSearch" />
              </template>
            </q-input>
          </div>
          <div v-if="!props.hideCreateButtons" class="col-12 col-sm-4 text-center" :class="{'text-right': !$q.screen.xs, 'q-mt-sm': $q.screen.xs}">
            <CreateTaskModalButton :task-group="props.taskGroup" @on-create="onCreate" />
          </div>
        </div>
      </template>

      <template v-slot:body="props">
        <q-tr :props="props" valign="top">
          <q-td key="id" :props="props">
            <q-btn flat icon="content_copy" size="sm" @click="toClipboard('Id', props.row.id)" />
            {{ props.row.id }}
          </q-td>

          <q-td key="name" :props="props">
            {{ props.row.name }}
          </q-td>

          <q-td key="worker" :props="props">
            {{ props.row.worker }}
          </q-td>

          <q-td key="isComplete" :props="props">
            {{ props.row.isComplete ? 'Yes' : 'No' }}
          </q-td>

          <q-td key="actions" :props="props">
            <CreateTaskModalButton label="" size="sm" flat :task-group="rootProps.taskGroup" :parent-id="props.row.id" @on-create="onCreate" />
            <EditTaskModalButton label="" size="sm" flat :task-group="rootProps.taskGroup" :task="props.row" @on-update="onUpdate" />
            <DeleteTaskModalButton label="" size="sm" flat :task="props.row" @on-delete="onDelete" />
          </q-td>
        </q-tr>
      </template>
    </q-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import toClipboard from 'src/lib/toClipboard'
import { useRouter, useRoute } from 'vue-router'
import notifyError from 'src/lib/notifyError'
import CreateTaskModalButton from 'src/components/task/CreateTaskModalButton.vue'
import EditTaskModalButton from 'src/components/task/EditTaskModalButton.vue'
import DeleteTaskModalButton from 'src/components/task/DeleteTaskModalButton.vue'
import { QTableProps } from 'quasar'
import { useTaskStore, Task } from 'src/stores/task-store'
import { TaskGroup } from 'src/stores/task-group-store'

const taskStore = useTaskStore()

const router = useRouter()
const route = useRoute()

interface Props {
  title?: string;
  taskGroup: TaskGroup;
  hideCreateButtons?: boolean;
  embedded?: boolean;
}
const props = withDefaults(defineProps<Props>(), { title: 'Tasks', hideCreateButtons: false, embedded: false })
const rootProps = props

const loading = ref(true)
const tasks = ref<Array<Task>>([])
const paginationModel = ref<QTableProps['pagination']>({
  page: parseInt(router.currentRoute.value.query.page ? router.currentRoute.value.query.page as string : '1'),
  rowsPerPage: router.currentRoute.value.query.page_size ? parseInt(router.currentRoute.value.query.page_size as string) : 20,
  rowsNumber: 0
})
const search = ref(router.currentRoute.value.query.q as string || '')

const columns : QTableProps['columns'] = [
  {
    name: 'id',
    field: 'id',
    label: 'Id',
    align: 'left'
  },
  {
    name: 'name',
    field: 'name',
    label: 'Name',
    align: 'left'
  },
  {
    name: 'worker',
    field: 'worker',
    label: 'Worker',
    align: 'left'
  },
  {
    name: 'isComplete',
    field: 'isComplete',
    label: 'Complete',
    align: 'left'
  },
  {
    name: 'actions',
    field: '',
    label: 'Actions',
    align: 'left'
  }
]

function onCreate (task: Task) {
  console.log('~~ task created', task)
  loadTasks()
}

function onUpdate (task: Task) {
  console.log('~~ task updated', task)
  loadTasks()
}

function onDelete (taskId: string) {
  console.log('~~ task deleted', taskId)
  loadTasks()
}

async function clearSearch () {
  search.value = ''
  await loadTasks()
}

const onRequest : QTableProps['onRequest'] = async ({ pagination }) => {
  paginationModel.value = pagination
  await loadTasks()
}

async function loadTasks () {
  try {
    if (paginationModel.value) {
      loading.value = true
      const result = await taskStore.getTasks(props.taskGroup.id, paginationModel.value.page, paginationModel.value.rowsPerPage, search.value)
      paginationModel.value.rowsNumber = result.count
      tasks.value = result.tasks
      router.push({ query: { ...route.query, q: search.value, page: paginationModel.value.page, page_size: paginationModel.value.rowsPerPage } })
    }
  } catch (e) {
    notifyError(e)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadTasks()
})
</script>
