import random

from locust import FastHttpUser, HttpUser, between

PRODUCT_COUNT = 1000  # the server seeds products 1..1000


def get_product(user):
    pid = random.randint(1, PRODUCT_COUNT)
    user.client.get(f"/products/{pid}", name="GET /products/[id]")


def post_details(user):
    pid = random.randint(1, PRODUCT_COUNT)
    body = {
        "product_id": pid,
        "sku": f"SKU-{pid:05d}",
        "manufacturer": "Acme Corporation",
        "category_id": random.randint(1, 50),
        "weight": random.randint(100, 5000),
        "some_other_id": random.randint(1, 1000),
    }
    user.client.post(f"/products/{pid}/details", json=body, name="POST /products/[id]/details")


# A store is read far more than it is written: 9 GETs for every POST.
MIX = {get_product: 9, post_details: 1}


class ProductHttpUser(HttpUser):
    tasks = MIX
    wait_time = between(0.1, 0.5)


class ProductFastHttpUser(FastHttpUser):
    tasks = MIX
    wait_time = between(0.1, 0.5)