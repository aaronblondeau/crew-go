import { copyToClipboard, Notify } from 'quasar'

export default async function toClipboard (name: string, value: string) {
  await copyToClipboard(value + '')
  Notify.create({
    type: 'positive',
    position: 'top',
    message: name + ' copied to clipboard.'
  })
}
