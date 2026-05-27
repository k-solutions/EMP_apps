import { createConsumer } from '@rails/actioncable'

// Automatically points to /cable
const consumer = createConsumer()

export default consumer
