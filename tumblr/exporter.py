import pytumblr


class Test:
    def test(self):
        client = pytumblr.TumblrRestClient()

        print(client.info())  # Grabs the current user information
