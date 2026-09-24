import threading

from golem.client import Client


class Channel:
    """Long-running input into Golem, such as a CLI or a chat app.

    Set ``id`` before ``run``. The process harness starts ``run`` on a
    background thread and sets ``stop`` on shutdown.

    Attributes:
        id: Channel name advertised to Golem, for example ``cli``.
    """

    id: str = ""

    def run(self, client: Client, stop: threading.Event) -> None:
        """Block until ``stop`` is set, reading user input and posting turns.

        Args:
            client: Client for this extension process.
            stop: Set when Golem is stopping the process.

        Raises:
            NotImplementedError: Override to advertise a channel loop.
        """
        raise NotImplementedError
