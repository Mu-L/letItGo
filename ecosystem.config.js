module.exports = {
    apps: [
        {
            name: 'api',
            script: 'go',
            args: 'run main.go',
            cwd: './api',
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
