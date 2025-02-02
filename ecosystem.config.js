module.exports = {
    apps: [
        {
            name: 'webhook',
            script: 'go',
            args: 'run main.go',
            cwd: './webhook',
            interpreter: 'none',
            output: './logs/api.log',
        },
        {
            name: 'producer',
            script: 'go',
            args: 'run main.go',
            cwd: './producer',
            interpreter: 'none',
            output: './logs/produce.log',
        },
        {
            name: 'consumer',
            script: 'go',
            args: 'run main.go',
            cwd: './consumer',
            interpreter: 'none',
            output: './logs/consumer.log',
        }
    ]
};
